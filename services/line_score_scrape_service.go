// File: services/line_score_scrape_service.go
package services

import (
	"log"

	"github.com/PuerkitoBio/goquery"
	"github.com/nprasad2077/NBA_Go/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FetchAndStoreLineScore parses the line score table from a goquery document and upserts it to the DB.
// It assumes the document has already been processed to make commented-out tables visible.
func FetchAndStoreLineScore(db *gorm.DB, doc *goquery.Document, gameID string) error {
	var lineScores []models.LineScore

	doc.Find("table#line_score tbody tr").Each(func(i int, row *goquery.Selection) {
		teamName := row.Find(`th[data-stat="team"] a`).Text()
		if teamName == "" {
			return // Skip invalid rows
		}

		ls := models.LineScore{
			GameID: gameID,
			Team:   teamName,
		}
		
		// Regular Quarters - no changes here
		ls.Q1 = mustAtoi(row.Find(`td[data-stat="1"]`).Text())
		ls.Q2 = mustAtoi(row.Find(`td[data-stat="2"]`).Text())
		ls.Q3 = mustAtoi(row.Find(`td[data-stat="3"]`).Text())
		ls.Q4 = mustAtoi(row.Find(`td[data-stat="4"]`).Text())

		// --- FIX: Corrected selectors for Overtime periods ---
		// The HTML uses "1OT", "2OT", etc., not "OT1", "OT2".
		ls.OT1 = mustAtoi(row.Find(`td[data-stat="1OT"]`).Text())
		ls.OT2 = mustAtoi(row.Find(`td[data-stat="2OT"]`).Text())
		ls.OT3 = mustAtoi(row.Find(`td[data-stat="3OT"]`).Text())
		
		// Total score
		ls.Total = mustAtoi(row.Find(`td[data-stat="T"]`).Text())
		
		lineScores = append(lineScores, ls)
	})

	if len(lineScores) == 0 {
		log.Printf("No line score data found to import for game %s.", gameID)
		return nil
	}

	// Upsert the data into the database.
	// Using a transaction is a good practice for atomicity.
	err := db.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "game_id"}, {Name: "team"}},
			DoUpdates: clause.AssignmentColumns([]string{"q1", "q2", "q3", "q4", "ot1", "ot2", "ot3", "total"}),
		}).Create(&lineScores).Error
	})

	if err != nil {
		log.Printf("Error upserting line scores for game %s: %v", gameID, err)
		return err
	}
	
	log.Printf("Successfully processed line scores for game %s.", gameID)
	return nil
}

