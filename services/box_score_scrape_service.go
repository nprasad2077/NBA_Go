package services

import (
	"context"
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
const numWorkers = 2 // Number of concurrent scrapers. Adjust based on your machine and network.
const baseDelay = 2500 * time.Millisecond

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

// FetchAndStoreBoxScoreDataForDateRange provides backward-compatibility by delegating
// to the chunked scraper with default 20-day chunks and 20s cool-off.
func FetchAndStoreBoxScoreDataForDateRange(db *gorm.DB, from, to time.Time) error {
	return FetchAndStoreBoxScoreDataChunked(context.Background(), db, from, to, 20, 20*time.Second, true)
}

// FetchAndStoreBoxScoreDataChunked processes box scores in temporal chunks (e.g. 20 days)
// with immediate per-chunk database upserts, cool-off periods, and graceful interrupt handling.
func FetchAndStoreBoxScoreDataChunked(
	ctx context.Context,
	db *gorm.DB,
	from, to time.Time,
	chunkDays int,
	coolOffBase time.Duration,
	skipExisting bool,
) error {
	if chunkDays <= 0 {
		chunkDays = 20
	}

	chunkDuration := time.Duration(chunkDays) * 24 * time.Hour
	totalDays := int(to.Sub(from).Hours()/24) + 1

	log.Printf("🚀 Starting Chunked Box Score Import: %s to %s (%d total days, %d days/chunk)",
		from.Format("2006-01-02"), to.Format("2006-01-02"), totalDays, chunkDays)

	currentStart := from
	chunkIndex := 1
	totalGamesScraped := 0

	for currentStart.Before(to) {
		select {
		case <-ctx.Done():
			log.Println("🛑 Interrupt received. Halting further chunk processing.")
			return ctx.Err()
		default:
		}

		currentEnd := currentStart.Add(chunkDuration)
		if currentEnd.After(to) {
			currentEnd = to
		}

		log.Printf("\n📦 ========================================================")
		log.Printf("📦 [Chunk %d] Window: %s to %s", chunkIndex, currentStart.Format("2006-01-02"), currentEnd.Format("2006-01-02"))
		log.Printf("📦 ========================================================")

		// 1. Query games for this chunk
		var games []models.Game
		query := db.Where("date >= ? AND date <= ?", currentStart, currentEnd.Add(24*time.Hour))
		if skipExisting {
			query = query.Where("game_id NOT IN (SELECT DISTINCT game_id FROM line_scores WHERE deleted_at IS NULL)")
		}

		if err := query.Order("date ASC").Find(&games).Error; err != nil {
			return fmt.Errorf("failed to query games for chunk %d: %w", chunkIndex, err)
		}

		if len(games) == 0 {
			log.Printf("⏩ [Chunk %d] All games in this window are already scraped or none found. Skipping.", chunkIndex)
			currentStart = currentEnd.Add(24 * time.Hour)
			chunkIndex++
			continue
		}

		log.Printf("Found %d pending games to scrape in Chunk %d. Starting worker pool...", len(games), chunkIndex)

		// 2. Concurrently scrape the chunk's games
		results := processGamesWithWorkers(ctx, games)

		// 3. IMMEDIATELY UPSERT chunk results to DB
		if len(results) > 0 {
			log.Printf("💾 Saving and upserting data for %d games from Chunk %d into PostgreSQL...", len(results), chunkIndex)
			if err := persistScrapedResults(db, results); err != nil {
				log.Printf("❌ Failed to upsert results for chunk %d: %v", chunkIndex, err)
				return err
			}
			totalGamesScraped += len(results)
			log.Printf("✅ [Chunk %d] Successfully saved %d games to database. (Total so far: %d)",
				chunkIndex, len(results), totalGamesScraped)
		}

		// Check if interrupted during chunk processing
		if ctx.Err() != nil {
			log.Printf("🛑 Process interrupted! All data scraped up to Chunk %d was safely committed to DB.", chunkIndex)
			return ctx.Err()
		}

		// 4. Cool-off pause between chunks
		if currentEnd.Before(to) {
			log.Printf("😴 Cool-off period: Pausing before Chunk %d...", chunkIndex+1)
			utils.SleepWithJitter(coolOffBase)
		}

		currentStart = currentEnd.Add(24 * time.Hour)
		chunkIndex++
	}

	log.Printf("\n🎉 All Box Score Chunks Finished! Successfully processed %d total games.", totalGamesScraped)
	return nil
}

// FetchAndStoreMissingBoxScores auto-detects all games in the database lacking line_scores,
// groups them into batches, and scrapes their box scores with rate limiting and immediate upserts.
func FetchAndStoreMissingBoxScores(
	ctx context.Context,
	db *gorm.DB,
	batchSize int,
	coolOffBase time.Duration,
) error {
	if batchSize <= 0 {
		batchSize = 20
	}

	var missingGames []models.Game
	err := db.Where("game_id NOT IN (SELECT DISTINCT game_id FROM line_scores WHERE deleted_at IS NULL)").
		Where("deleted_at IS NULL").
		Order("date ASC").
		Find(&missingGames).Error

	if err != nil {
		return fmt.Errorf("failed to query missing games: %w", err)
	}

	totalMissing := len(missingGames)
	if totalMissing == 0 {
		log.Println("🎉 All games in the database already have box score and line score data! Nothing to scrape.")
		return nil
	}

	totalBatches := (totalMissing + batchSize - 1) / batchSize
	log.Printf("🔍 Auto-Detect: Found %d missing games across all seasons. Processing in %d batches (%d games/batch).",
		totalMissing, totalBatches, batchSize)

	totalSaved := 0

	for i := 0; i < totalBatches; i++ {
		select {
		case <-ctx.Done():
			log.Println("🛑 Interrupt received. Halting missing box score scraping.")
			return ctx.Err()
		default:
		}

		startIdx := i * batchSize
		endIdx := startIdx + batchSize
		if endIdx > totalMissing {
			endIdx = totalMissing
		}

		batchGames := missingGames[startIdx:endIdx]
		batchNumber := i + 1

		log.Printf("\n📦 ========================================================")
		log.Printf("📦 [Batch %d/%d] Processing %d games (%s to %s)",
			batchNumber, totalBatches, len(batchGames),
			batchGames[0].Date.Format("2006-01-02"),
			batchGames[len(batchGames)-1].Date.Format("2006-01-02"))
		log.Printf("📦 ========================================================")

		// Scrape batch with worker pool
		results := processGamesWithWorkers(ctx, batchGames)

		// Immediate database upsert
		if len(results) > 0 {
			log.Printf("💾 Saving and upserting data for %d games from Batch %d into PostgreSQL...", len(results), batchNumber)
			if err := persistScrapedResults(db, results); err != nil {
				log.Printf("❌ Failed to upsert results for batch %d: %v", batchNumber, err)
				return err
			}
			totalSaved += len(results)
			log.Printf("✅ [Batch %d/%d] Successfully saved %d games. (Total progress: %d/%d)",
				batchNumber, totalBatches, len(results), totalSaved, totalMissing)
		}

		if ctx.Err() != nil {
			log.Printf("🛑 Process interrupted! All data scraped up to Batch %d was safely committed to DB.", batchNumber)
			return ctx.Err()
		}

		// Cool-off pause between batches
		if i < totalBatches-1 {
			log.Printf("😴 Cool-off period: Pausing before Batch %d/%d...", batchNumber+1, totalBatches)
			utils.SleepWithJitter(coolOffBase)
		}
	}

	log.Printf("\n🎉 All Missing Box Scores Finished! Successfully processed %d total games.", totalSaved)
	return nil
}

// processGamesWithWorkers runs the worker pool for a slice of games.
func processGamesWithWorkers(ctx context.Context, games []models.Game) []ScrapedResult {
	jobs := make(chan models.Game, len(games))
	resultsChan := make(chan ScrapedResult, len(games))
	var wg sync.WaitGroup

	// Start worker goroutines
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go scrapeAndParseWorker(ctx, w, jobs, resultsChan, &wg)
	}

	// Send jobs to the workers
	for _, game := range games {
		select {
		case <-ctx.Done():
			break
		case jobs <- game:
		}
	}
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()
	close(resultsChan)

	var validResults []ScrapedResult
	for res := range resultsChan {
		if res.Err != nil {
			log.Printf("⚠️ Worker failed on game %s: %v", res.GameID, res.Err)
			continue
		}
		validResults = append(validResults, res)
	}
	return validResults
}

// persistScrapedResults extracts and batch-upserts line scores, player stats, and team stats.
func persistScrapedResults(db *gorm.DB, results []ScrapedResult) error {
	var allPlayerBasicStats []models.PlayerGameBasicStat
	var allPlayerAdvStats []models.PlayerGameAdvStat
	var allTeamBasicStats []models.TeamGameBasicStat
	var allTeamAdvStats []models.TeamGameAdvStat
	var allLineScores []models.LineScore

	for _, res := range results {
		allPlayerBasicStats = append(allPlayerBasicStats, res.PlayerBasicStats...)
		allPlayerAdvStats = append(allPlayerAdvStats, res.PlayerAdvStats...)
		allTeamBasicStats = append(allTeamBasicStats, res.TeamBasicStats...)
		allTeamAdvStats = append(allTeamAdvStats, res.TeamAdvStats...)
		allLineScores = append(allLineScores, res.LineScores...)
	}

	// Upsert Line Scores
	if len(allLineScores) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "game_id"}, {Name: "team"}},
			DoUpdates: clause.AssignmentColumns(getModelColumns(&models.LineScore{})),
		}).Create(&allLineScores).Error; err != nil {
			return fmt.Errorf("failed to upsert line scores: %w", err)
		}
	}

	// Batch upsert Player and Team Stats
	if err := batchUpsertAll(db, allPlayerBasicStats, allPlayerAdvStats, allTeamBasicStats, allTeamAdvStats); err != nil {
		return fmt.Errorf("batch upsert failed: %w", err)
	}

	return nil
}

// scrapeAndParseWorker is a worker goroutine that receives games, scrapes them, and sends back the result.
func scrapeAndParseWorker(ctx context.Context, id int, jobs <-chan models.Game, results chan<- ScrapedResult, wg *sync.WaitGroup) {
	defer wg.Done()

	// Stagger worker start
	if numWorkers > 1 {
		staggerAmount := time.Duration(int64(baseDelay) / int64(numWorkers))
		initialDelay := time.Duration(id-1) * staggerAmount
		log.Printf("Worker %d: Staggering start with an initial delay of %v", id, initialDelay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(initialDelay):
		}
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d: Interrupted, finishing up...", id)
			return
		case game, ok := <-jobs:
			if !ok {
				return
			}

			boxScoreURL := game.BoxScoreURL
			if boxScoreURL == "" {
				boxScoreURL = fmt.Sprintf("/boxscores/%s.html", game.GameID)
			}
			fullURL := boxScoreURLBase + boxScoreURL

			utils.SleepWithJitter(baseDelay)
			time.Sleep(2500 * time.Millisecond)

			select {
			case <-ctx.Done():
				return
			default:
			}

			req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
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
			OT1:    mustAtoi(row.Find(`td[data-stat="1OT"]`).Text()),
			OT2:    mustAtoi(row.Find(`td[data-stat="2OT"]`).Text()),
			OT3:    mustAtoi(row.Find(`td[data-stat="3OT"]`).Text()),
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
	const batchSize = 500 // 💡 A safe chunk size to stay under the limit.

	// Batch upsert Player Basic Stats
	if len(pbs) > 0 {
		for i := 0; i < len(pbs); i += batchSize {
			end := i + batchSize
			if end > len(pbs) {
				end = len(pbs)
			}
			batch := pbs[i:end]
			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "game_id"}, {Name: "player_id"}},
				DoUpdates: clause.AssignmentColumns(getModelColumns(&models.PlayerGameBasicStat{})),
			}).Create(&batch).Error; err != nil {
				return fmt.Errorf("failed to upsert player basic stats batch: %w", err)
			}
		}
		log.Printf("Successfully upserted %d player basic stats.", len(pbs))
	}

	// Batch upsert Player Advanced Stats
	if len(pas) > 0 {
		for i := 0; i < len(pas); i += batchSize {
			end := i + batchSize
			if end > len(pas) {
				end = len(pas)
			}
			batch := pas[i:end]
			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "game_id"}, {Name: "player_id"}},
				DoUpdates: clause.AssignmentColumns(getModelColumns(&models.PlayerGameAdvStat{})),
			}).Create(&batch).Error; err != nil {
				return fmt.Errorf("failed to upsert player advanced stats batch: %w", err)
			}
		}
		log.Printf("Successfully upserted %d player advanced stats.", len(pas))
	}

	// Batch upsert Team Basic Stats
	if len(tbs) > 0 {
		for i := 0; i < len(tbs); i += batchSize {
			end := i + batchSize
			if end > len(tbs) {
				end = len(tbs)
			}
			batch := tbs[i:end]
			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "game_id"}, {Name: "team"}},
				DoUpdates: clause.AssignmentColumns(getModelColumns(&models.TeamGameBasicStat{})),
			}).Create(&batch).Error; err != nil {
				return fmt.Errorf("failed to upsert team basic stats batch: %w", err)
			}
		}
		log.Printf("Successfully upserted %d team basic stats.", len(tbs))
	}

	// Batch upsert Team Advanced Stats
	if len(tas) > 0 {
		for i := 0; i < len(tas); i += batchSize {
			end := i + batchSize
			if end > len(tas) {
				end = len(tas)
			}
			batch := tas[i:end]
			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "game_id"}, {Name: "team"}},
				DoUpdates: clause.AssignmentColumns(getModelColumns(&models.TeamGameAdvStat{})),
			}).Create(&batch).Error; err != nil {
				return fmt.Errorf("failed to upsert team advanced stats batch: %w", err)
			}
		}
		log.Printf("Successfully upserted %d team advanced stats.", len(tas))
	}

	return nil
}
