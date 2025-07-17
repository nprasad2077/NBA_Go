package services

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const boxScoreURLBase = "https://www.basketball-reference.com"
const numWorkers = 8 // Number of concurrent scrapers. Adjust based on your machine and network.

// ScrapedResult holds all the parsed stats from a single game.
type ScrapedResult struct {
	PlayerBasicStats []models.PlayerGameBasicStat
	PlayerAdvStats   []models.PlayerGameAdvStat
	TeamBasicStats   []models.TeamGameBasicStat
	TeamAdvStats     []models.TeamGameAdvStat
	LineScores       []models.LineScore
	GameID           string
	Err              error
}

// uncommentDoc finds and replaces commented out HTML sections.
func uncommentDoc(doc *goquery.Document) *goquery.Document {
	doc.Find("*").Contents().FilterFunction(func(i int, s *goquery.Selection) bool {
		return goquery.NodeName(s) == "#comment"
	}).Each(func(i int, s *goquery.Selection) {
		commentText := s.Nodes[0].Data
		if strings.Contains(commentText, "<table") {
			s.ReplaceWithHtml(commentText)
		}
	})
	return doc
}

// FetchAndStoreBoxScoreDataForDateRange fetches games and batch processes their box scores concurrently.
func FetchAndStoreBoxScoreDataForDateRange(db *gorm.DB, from, to time.Time) error {
	var games []models.Game
	if err := db.Where("date >= ? AND date < ?", from, to.Add(24*time.Hour)).Find(&games).Error; err != nil {
		return fmt.Errorf("failed to query games from DB: %w", err)
	}

	if len(games) == 0 {
		log.Println("No games found to process in the specified date range.")
		return nil
	}
	log.Printf("Found %d games to process. Initializing concurrent scraping...", len(games))

	// --- Concurrency Setup ---
	jobs := make(chan models.Game, len(games))
	results := make(chan ScrapedResult, len(games))
	var wg sync.WaitGroup

	// Start worker goroutines
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go scrapeAndParseWorker(w, jobs, results, &wg)
	}

	// Send jobs to the workers
	for _, game := range games {
		jobs <- game
	}
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()
	close(results)

	// --- Aggregation & Final Upsert ---
	log.Println("All scraping complete. Aggregating results for final batch upsert...")
	var allPlayerBasicStats []models.PlayerGameBasicStat
	var allPlayerAdvStats []models.PlayerGameAdvStat
	var allTeamBasicStats []models.TeamGameBasicStat
	var allTeamAdvStats []models.TeamGameAdvStat
	var allLineScores []models.LineScore

	for res := range results {
		if res.Err != nil {
			log.Printf("A worker failed on game %s: %v", res.GameID, res.Err)
			continue
		}
		allPlayerBasicStats = append(allPlayerBasicStats, res.PlayerBasicStats...)
		allPlayerAdvStats = append(allPlayerAdvStats, res.PlayerAdvStats...)
		allTeamBasicStats = append(allTeamBasicStats, res.TeamBasicStats...)
		allTeamAdvStats = append(allTeamAdvStats, res.TeamAdvStats...)
		allLineScores = append(allLineScores, res.LineScores...)
	}

	// Upsert Line Scores first
	if len(allLineScores) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "game_id"}, {Name: "team"}},
			DoUpdates: clause.AssignmentColumns(getModelColumns(&models.LineScore{})),
		}).Create(&allLineScores).Error; err != nil {
			return fmt.Errorf("failed to upsert line scores: %w", err)
		}
		log.Printf("Successfully upserted %d line scores.", len(allLineScores))
	}

	// Call the batch upsert function with the fully aggregated data
	if err := batchUpsertAll(db, allPlayerBasicStats, allPlayerAdvStats, allTeamBasicStats, allTeamAdvStats); err != nil {
		return fmt.Errorf("final batch upsert failed: %w", err)
	}

	log.Printf("Successfully upserted all box score data for %d games.", len(games))
	return nil
}

// scrapeAndParseWorker is a worker goroutine that receives games, scrapes them, and sends back the result.
func scrapeAndParseWorker(id int, jobs <-chan models.Game, results chan<- ScrapedResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for game := range jobs {
		log.Printf("Worker %d: Processing game %s", id, game.GameID)
		fullURL := boxScoreURLBase + game.BoxScoreURL

		utils.SleepWithJitter(2300 * time.Millisecond)

		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			results <- ScrapedResult{GameID: game.GameID, Err: fmt.Errorf("failed to create request: %w", err)}
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			results <- ScrapedResult{GameID: game.GameID, Err: fmt.Errorf("request failed: %w", err)}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			results <- ScrapedResult{GameID: game.GameID, Err: fmt.Errorf("received non-200 status code: %s", resp.Status)}
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			results <- ScrapedResult{GameID: game.GameID, Err: fmt.Errorf("failed to parse document: %w", err)}
			continue
		}

		doc = uncommentDoc(doc)

		lineScores := parseLineScore(doc, game.GameID)
		pbs, pas, tbs, tas := parseBoxScores(doc, game.GameID)

		results <- ScrapedResult{
			PlayerBasicStats: pbs,
			PlayerAdvStats:   pas,
			TeamBasicStats:   tbs,
			TeamAdvStats:     tas,
			LineScores:       lineScores,
			GameID:           game.GameID,
			Err:              nil,
		}
	}
}

// parseBoxScores now returns the slices instead of calling the DB.
func parseBoxScores(doc *goquery.Document, gameID string) ([]models.PlayerGameBasicStat, []models.PlayerGameAdvStat, []models.TeamGameBasicStat, []models.TeamGameAdvStat) {
	var allPlayerBasicStats []models.PlayerGameBasicStat
	var allPlayerAdvStats []models.PlayerGameAdvStat
	var allTeamBasicStats []models.TeamGameBasicStat
	var allTeamAdvStats []models.TeamGameAdvStat

	doc.Find(`table[id^="box-"][id$="-game-basic"], table[id^="box-"][id$="-game-advanced"]`).Each(func(i int, table *goquery.Selection) {
		tableID, _ := table.Attr("id")
		isAdvanced := strings.Contains(tableID, "-advanced")

		teamAbbr := strings.TrimSuffix(strings.TrimPrefix(tableID, "box-"), "-game-basic")
		teamAbbr = strings.TrimSuffix(teamAbbr, "-game-advanced")

		table.Find("tbody tr").Each(func(j int, row *goquery.Selection) {
			playerID, exists := row.Find("th").Attr("data-append-csv")
			if !exists || playerID == "" {
				return
			}

			status := "Played"
			if reason := row.Find(`td[data-stat="reason"]`); reason.Length() > 0 {
				status = reason.Text()
			}

			if !isAdvanced {
				allPlayerBasicStats = append(allPlayerBasicStats, parsePlayerBasicStat(row, gameID, playerID, teamAbbr, status))
			} else {
				allPlayerAdvStats = append(allPlayerAdvStats, parsePlayerAdvStat(row, gameID, playerID, teamAbbr))
			}
		})

		table.Find("tfoot tr").Each(func(j int, row *goquery.Selection) {
			if !isAdvanced {
				allTeamBasicStats = append(allTeamBasicStats, parseTeamBasicStat(row, gameID, teamAbbr))
			} else {
				allTeamAdvStats = append(allTeamAdvStats, parseTeamAdvStat(row, gameID, teamAbbr))
			}
		})
	})

	return allPlayerBasicStats, allPlayerAdvStats, allTeamBasicStats, allTeamAdvStats
}

func parseLineScore(doc *goquery.Document, gameID string) []models.LineScore {
	var lineScores []models.LineScore
	doc.Find("#line_score tbody tr").Each(func(i int, row *goquery.Selection) {
		teamAbbr := row.Find(`th a`).Text()
		if teamAbbr == "" {
			return
		}

		lineScores = append(lineScores, models.LineScore{
			GameID: gameID,
			Team:   teamAbbr,
			Q1:     mustAtoi(row.Find(`td[data-stat="1"]`).Text()),
			Q2:     mustAtoi(row.Find(`td[data-stat="2"]`).Text()),
			Q3:     mustAtoi(row.Find(`td[data-stat="3"]`).Text()),
			Q4:     mustAtoi(row.Find(`td[data-stat="4"]`).Text()),
			OT1:    mustAtoi(row.Find(`td[data-stat="OT1"]`).Text()),
			OT2:    mustAtoi(row.Find(`td[data-stat="OT2"]`).Text()),
			OT3:    mustAtoi(row.Find(`td[data-stat="OT3"]`).Text()),
			Total:  mustAtoi(row.Find(`td[data-stat="T"]`).Text()),
		})
	})
	return lineScores
}

func parsePlayerBasicStat(row *goquery.Selection, gameID, playerID, team, status string) models.PlayerGameBasicStat {
	return models.PlayerGameBasicStat{
		GameID:        gameID, PlayerID: playerID, Team: team, Status: status,
		PlayerName:    row.Find(`th[data-stat="player"] a`).Text(),
		MP:            row.Find(`td[data-stat="mp"]`).Text(),
		FG:            mustAtoi(row.Find(`td[data-stat="fg"]`).Text()),
		FGA:           mustAtoi(row.Find(`td[data-stat="fga"]`).Text()),
		FGPercent:     mustParseFloat(row.Find(`td[data-stat="fg_pct"]`).Text()),
		ThreeP:        mustAtoi(row.Find(`td[data-stat="fg3"]`).Text()),
		ThreePA:       mustAtoi(row.Find(`td[data-stat="fg3a"]`).Text()),
		ThreePPercent: mustParseFloat(row.Find(`td[data-stat="fg3_pct"]`).Text()),
		FT:            mustAtoi(row.Find(`td[data-stat="ft"]`).Text()),
		FTA:           mustAtoi(row.Find(`td[data-stat="fta"]`).Text()),
		FTPercent:     mustParseFloat(row.Find(`td[data-stat="ft_pct"]`).Text()),
		ORB:           mustAtoi(row.Find(`td[data-stat="orb"]`).Text()),
		DRB:           mustAtoi(row.Find(`td[data-stat="drb"]`).Text()),
		TRB:           mustAtoi(row.Find(`td[data-stat="trb"]`).Text()),
		AST:           mustAtoi(row.Find(`td[data-stat="ast"]`).Text()),
		STL:           mustAtoi(row.Find(`td[data-stat="stl"]`).Text()),
		BLK:           mustAtoi(row.Find(`td[data-stat="blk"]`).Text()),
		TOV:           mustAtoi(row.Find(`td[data-stat="tov"]`).Text()),
		PF:            mustAtoi(row.Find(`td[data-stat="pf"]`).Text()),
		PTS:           mustAtoi(row.Find(`td[data-stat="pts"]`).Text()),
		GmSc:          mustParseFloat(row.Find(`td[data-stat="game_score"]`).Text()),
		PlusMinus:     mustAtoiWithSign(row.Find(`td[data-stat="plus_minus"]`).Text()), // <-- UPDATED LINE
	}
}

func parsePlayerAdvStat(row *goquery.Selection, gameID, playerID, team string) models.PlayerGameAdvStat {
	return models.PlayerGameAdvStat{
		GameID: gameID, PlayerID: playerID, Team: team,
		PlayerName: row.Find(`th[data-stat="player"] a`).Text(),
		MP:         row.Find(`td[data-stat="mp"]`).Text(),
		TSPercent:  mustParseFloat(row.Find(`td[data-stat="ts_pct"]`).Text()),
		EFGPercent: mustParseFloat(row.Find(`td[data-stat="efg_pct"]`).Text()),
		ThreePAr:   mustParseFloat(row.Find(`td[data-stat="fg3a_per_fga_pct"]`).Text()),
		FTr:        mustParseFloat(row.Find(`td[data-stat="fta_per_fga_pct"]`).Text()),
		ORBPercent: mustParseFloat(row.Find(`td[data-stat="orb_pct"]`).Text()),
		DRBPercent: mustParseFloat(row.Find(`td[data-stat="drb_pct"]`).Text()),
		TRBPercent: mustParseFloat(row.Find(`td[data-stat="trb_pct"]`).Text()),
		ASTPercent: mustParseFloat(row.Find(`td[data-stat="ast_pct"]`).Text()),
		STLPercent: mustParseFloat(row.Find(`td[data-stat="stl_pct"]`).Text()),
		BLKPercent: mustParseFloat(row.Find(`td[data-stat="blk_pct"]`).Text()),
		TOVPercent: mustParseFloat(row.Find(`td[data-stat="tov_pct"]`).Text()),
		USGPercent: mustParseFloat(row.Find(`td[data-stat="usg_pct"]`).Text()),
		ORtg:       mustAtoi(row.Find(`td[data-stat="off_rtg"]`).Text()),
		DRtg:       mustAtoi(row.Find(`td[data-stat="def_rtg"]`).Text()),
		BPM:        mustParseFloat(row.Find(`td[data-stat="bpm"]`).Text()),
	}
}

func parseTeamBasicStat(row *goquery.Selection, gameID, team string) models.TeamGameBasicStat {
	return models.TeamGameBasicStat{
		GameID: gameID, Team: team,
		MP:            mustAtoi(row.Find(`td[data-stat="mp"]`).Text()),
		FG:            mustAtoi(row.Find(`td[data-stat="fg"]`).Text()),
		FGA:           mustAtoi(row.Find(`td[data-stat="fga"]`).Text()),
		FGPercent:     mustParseFloat(row.Find(`td[data-stat="fg_pct"]`).Text()),
		ThreeP:        mustAtoi(row.Find(`td[data-stat="fg3"]`).Text()),
		ThreePA:       mustAtoi(row.Find(`td[data-stat="fg3a"]`).Text()),
		ThreePPercent: mustParseFloat(row.Find(`td[data-stat="fg3_pct"]`).Text()),
		FT:            mustAtoi(row.Find(`td[data-stat="ft"]`).Text()),
		FTA:           mustAtoi(row.Find(`td[data-stat="fta"]`).Text()),
		FTPercent:     mustParseFloat(row.Find(`td[data-stat="ft_pct"]`).Text()),
		ORB:           mustAtoi(row.Find(`td[data-stat="orb"]`).Text()),
		DRB:           mustAtoi(row.Find(`td[data-stat="drb"]`).Text()),
		TRB:           mustAtoi(row.Find(`td[data-stat="trb"]`).Text()),
		AST:           mustAtoi(row.Find(`td[data-stat="ast"]`).Text()),
		STL:           mustAtoi(row.Find(`td[data-stat="stl"]`).Text()),
		BLK:           mustAtoi(row.Find(`td[data-stat="blk"]`).Text()),
		TOV:           mustAtoi(row.Find(`td[data-stat="tov"]`).Text()),
		PF:            mustAtoi(row.Find(`td[data-stat="pf"]`).Text()),
		PTS:           mustAtoi(row.Find(`td[data-stat="pts"]`).Text()),
	}
}

func parseTeamAdvStat(row *goquery.Selection, gameID, team string) models.TeamGameAdvStat {
	return models.TeamGameAdvStat{
		GameID: gameID, Team: team,
		MP:         mustAtoi(row.Find(`td[data-stat="mp"]`).Text()),
		TSPercent:  mustParseFloat(row.Find(`td[data-stat="ts_pct"]`).Text()),
		EFGPercent: mustParseFloat(row.Find(`td[data-stat="efg_pct"]`).Text()),
		ThreePAr:   mustParseFloat(row.Find(`td[data-stat="fg3a_per_fga_pct"]`).Text()),
		FTr:        mustParseFloat(row.Find(`td[data-stat="fta_per_fga_pct"]`).Text()),
		ORBPercent: mustParseFloat(row.Find(`td[data-stat="orb_pct"]`).Text()),
		DRBPercent: mustParseFloat(row.Find(`td[data-stat="drb_pct"]`).Text()),
		TRBPercent: mustParseFloat(row.Find(`td[data-stat="trb_pct"]`).Text()),
		ASTPercent: mustParseFloat(row.Find(`td[data-stat="ast_pct"]`).Text()),
		STLPercent: mustParseFloat(row.Find(`td[data-stat="stl_pct"]`).Text()),
		BLKPercent: mustParseFloat(row.Find(`td[data-stat="blk_pct"]`).Text()),
		TOVPercent: mustParseFloat(row.Find(`td[data-stat="tov_pct"]`).Text()),
		USGPercent: mustParseFloat(row.Find(`td[data-stat="usg_pct"]`).Text()),
		ORtg:       mustParseFloat(row.Find(`td[data-stat="off_rtg"]`).Text()),
		DRtg:       mustParseFloat(row.Find(`td[data-stat="def_rtg"]`).Text()),
	}
}

func batchUpsertAll(db *gorm.DB, pbs []models.PlayerGameBasicStat, pas []models.PlayerGameAdvStat, tbs []models.TeamGameBasicStat, tas []models.TeamGameAdvStat) error {
	if len(pbs) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "game_id"}, {Name: "player_id"}},
			DoUpdates: clause.AssignmentColumns(getModelColumns(&models.PlayerGameBasicStat{})),
		}).Create(&pbs).Error; err != nil {
			return fmt.Errorf("failed to upsert player basic stats: %w", err)
		}
	}
	if len(pas) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "game_id"}, {Name: "player_id"}},
			DoUpdates: clause.AssignmentColumns(getModelColumns(&models.PlayerGameAdvStat{})),
		}).Create(&pas).Error; err != nil {
			return fmt.Errorf("failed to upsert player advanced stats: %w", err)
		}
	}
	if len(tbs) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "game_id"}, {Name: "team"}},
			DoUpdates: clause.AssignmentColumns(getModelColumns(&models.TeamGameBasicStat{})),
		}).Create(&tbs).Error; err != nil {
			return fmt.Errorf("failed to upsert team basic stats: %w", err)
		}
	}
	if len(tas) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "game_id"}, {Name: "team"}},
			DoUpdates: clause.AssignmentColumns(getModelColumns(&models.TeamGameAdvStat{})),
		}).Create(&tas).Error; err != nil {
			return fmt.Errorf("failed to upsert team advanced stats: %w", err)
		}
	}
	return nil
}