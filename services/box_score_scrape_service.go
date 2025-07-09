package services

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const boxScoreURLBase = "https://www.basketball-reference.com"

// uncommentDoc finds and replaces commented out HTML sections.
func uncommentDoc(doc *goquery.Document) *goquery.Document {
	doc.Find("*").Contents().FilterFunction(func(i int, s *goquery.Selection) bool {
		return goquery.NodeName(s) == "#comment"
	}).Each(func(i int, s *goquery.Selection) {
		// Use .Data on the underlying html.Node to get the comment content.
		commentText := s.Nodes[0].Data
		if strings.Contains(commentText, "<table") {
			// Replace the comment node with its content.
			s.ReplaceWithHtml(commentText)
		}
	})
	return doc
}

// FetchAndStoreBoxScoreDataForDateRange fetches all games in a date range and scrapes their box scores.
func FetchAndStoreBoxScoreDataForDateRange(db *gorm.DB, from, to time.Time) error {
	var games []models.Game
	// Query the database for games within the specified date range.
	if err := db.Where("date >= ? AND date < ?", from, to).Find(&games).Error; err != nil {
		return fmt.Errorf("failed to query games from DB: %w", err)
	}

	log.Printf("Found %d games to process in the specified date range.", len(games))

	for _, game := range games {
		log.Printf("Processing game: %s", game.GameID)
		fullURL := boxScoreURLBase + game.BoxScoreURL
		if err := scrapeBoxScorePage(db, fullURL, game.GameID); err != nil {
			// Log the error but continue to the next game
			log.Printf("Error processing box score for game %s: %v", game.GameID, err)
		}
		// Be a good internet citizen and pause between requests.
		utils.SleepWithJitter(3000 * time.Millisecond)
	}
	return nil
}

// scrapeBoxScorePage handles fetching and parsing a single box score page.
func scrapeBoxScorePage(db *gorm.DB, url, gameID string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code: %s", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	// 1. Uncomment all tables in the document first.
	doc = uncommentDoc(doc)

	// 2. Call the dedicated service to handle line scores.
	if err := FetchAndStoreLineScore(db, doc, gameID); err != nil {
		log.Printf("Error processing line score for game %s: %v", gameID, err)
	}

	// 3. The existing box score parser will now work because its tables are visible.
	if err := parseAndStoreBoxScores(db, doc, gameID); err != nil {
		return err
	}

	return nil
}

// parseAndStoreBoxScores finds all basic and advanced box score tables and processes them.
func parseAndStoreBoxScores(db *gorm.DB, doc *goquery.Document, gameID string) error {
	var allPlayerBasicStats []models.PlayerGameBasicStat
	var allPlayerAdvStats []models.PlayerGameAdvStat
	var allTeamBasicStats []models.TeamGameBasicStat
	var allTeamAdvStats []models.TeamGameAdvStat

	// Use a CSS attribute selector to find all box score tables for both teams.
	doc.Find(`table[id^="box-"][id$="-game-basic"], table[id^="box-"][id$="-game-advanced"]`).Each(func(i int, table *goquery.Selection) {
		tableID, _ := table.Attr("id")
		isAdvanced := strings.Contains(tableID, "-advanced")
		teamAbbr := strings.TrimSuffix(strings.TrimPrefix(tableID, "box-"), "-game-basic")
		teamAbbr = strings.TrimSuffix(teamAbbr, "-game-advanced")

		// Process player rows
		table.Find("tbody tr").Each(func(j int, row *goquery.Selection) {
			playerID, exists := row.Find("th").Attr("data-append-csv")
			if !exists || playerID == "" {
				return // Not a player row
			}

			// Handle "Did Not Play" or other statuses
			reason := row.Find(`td[data-stat="reason"]`)
			status := "Played"
			if reason.Length() > 0 {
				status = reason.Text()
			}

			if !isAdvanced {
				stat := parsePlayerBasicStat(row, gameID, playerID, teamAbbr, status)
				allPlayerBasicStats = append(allPlayerBasicStats, stat)
			} else {
				stat := parsePlayerAdvStat(row, gameID, playerID, teamAbbr, status)
				allPlayerAdvStats = append(allPlayerAdvStats, stat)
			}
		})

		// Process team total row
		table.Find("tfoot tr").Each(func(j int, row *goquery.Selection) {
			if !isAdvanced {
				stat := parseTeamBasicStat(row, gameID, teamAbbr)
				allTeamBasicStats = append(allTeamBasicStats, stat)
			} else {
				stat := parseTeamAdvStat(row, gameID, teamAbbr)
				allTeamAdvStats = append(allTeamAdvStats, stat)
			}
		})
	})

	// Batch upsert all collected stats
	if err := batchUpsertAll(db, allPlayerBasicStats, allPlayerAdvStats, allTeamBasicStats, allTeamAdvStats); err != nil {
		return err
	}

	return nil
}

// --- Parsing Helper Functions ---

func parsePlayerBasicStat(row *goquery.Selection, gameID, playerID, team, status string) models.PlayerGameBasicStat {
	return models.PlayerGameBasicStat{
		GameID:        gameID,
		PlayerID:      playerID,
		PlayerName:    row.Find(`th[data-stat="player"] a`).Text(),
		Team:          team,
		Status:        status,
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
		PlusMinus:     mustAtoiWithSign(row.Find(`td[data-stat="plus_minus"]`).Text()),
	}
}

func parsePlayerAdvStat(row *goquery.Selection, gameID, playerID, team, status string) models.PlayerGameAdvStat {
	return models.PlayerGameAdvStat{
		GameID:     gameID,
		PlayerID:   playerID,
		PlayerName: row.Find(`th[data-stat="player"] a`).Text(),
		Team:       team,
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
		GameID:        gameID,
		Team:          team,
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
		GameID:     gameID,
		Team:       team,
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

// --- DB and Utility Functions ---

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



// getModelColumns is a placeholder for a more robust reflection-based column name generator.
// For now, it returns hardcoded lists.
func getModelColumns(model interface{}) []string {
	switch model.(type) {
	case *models.PlayerGameBasicStat:
		return []string{"player_name", "team", "status", "mp", "fg", "fga", "fg_percent", "three_p", "three_pa", "three_p_percent", "ft", "fta", "ft_percent", "orb", "drb", "trb", "ast", "stl", "blk", "tov", "pf", "pts", "gm_sc", "plus_minus"}
	case *models.PlayerGameAdvStat:
		return []string{"player_name", "team", "mp", "ts_percent", "efg_percent", "three_p_ar", "f_tr", "orb_percent", "drb_percent", "trb_percent", "ast_percent", "stl_percent", "blk_percent", "tov_percent", "usg_percent", "o_rtg", "d_rtg", "bpm"}
	case *models.TeamGameBasicStat:
		return []string{"mp", "fg", "fga", "fg_percent", "three_p", "three_pa", "three_p_percent", "ft", "fta", "ft_percent", "orb", "drb", "trb", "ast", "stl", "blk", "tov", "pf", "pts"}
	case *models.TeamGameAdvStat:
		return []string{"mp", "ts_percent", "efg_percent", "three_p_ar", "f_tr", "orb_percent", "drb_percent", "trb_percent", "ast_percent", "stl_percent", "blk_percent", "tov_percent", "usg_percent", "o_rtg", "d_rtg"}
	}
	return []string{}
}
