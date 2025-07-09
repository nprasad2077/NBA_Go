package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// LineScore struct and mustAtoi helper function remain the same
type LineScore struct {
	GameID string
	Team   string
	Q1, Q2, Q3, Q4 int
	OT1, OT2, OT3  int
	Total          int
}
func mustAtoi(s string) int { i, _ := strconv.Atoi(s); return i }


func scrapeAndPrintLineScore(gameURL, gameID string) {
	// 1. Fetch the page
	resp, err := http.Get(gameURL)
	if err != nil {
		log.Fatalf("Failed to fetch page: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
	if err != nil {
		log.Fatalf("Failed to parse HTML: %v", err)
	}

	var table *goquery.Selection
	
	// Try to find a visible table first
	table = doc.Find("table#line_score")

	if table.Length() == 0 {
		log.Println("Visible table not found. Finding and parsing commented table...")
		
		commentNode := doc.Find("#all_line_score").Contents().FilterFunction(func(i int, s *goquery.Selection) bool {
			return goquery.NodeName(s) == "#comment"
		})

		if commentNode.Length() > 0 {
			// ✅ THE FIX: Access the .Data field directly from the comment node.
			commentedHTML := commentNode.Nodes[0].Data
			
			innerDoc, err := goquery.NewDocumentFromReader(strings.NewReader(commentedHTML))
			if err != nil {
				log.Fatalf("❌ Failed to parse the HTML from the comment: %v", err)
			}
			table = innerDoc.Find("table#line_score")
		}
	}

	if table == nil || table.Length() == 0 {
		log.Println("❌ No line score table found by any method.")
		return
	}

	// 3. Parse the data from the found table
	var lineScores []LineScore
	table.Find("tbody tr").Each(func(i int, row *goquery.Selection) {
		teamName := row.Find(`th[data-stat="team"] a`).Text()
		if teamName != "" {
			ls := LineScore{GameID: gameID, Team: teamName}
			ls.Q1 = mustAtoi(row.Find(`td[data-stat="1"]`).Text())
			ls.Q2 = mustAtoi(row.Find(`td[data-stat="2"]`).Text())
			ls.Q3 = mustAtoi(row.Find(`td[data-stat="3"]`).Text())
			ls.Q4 = mustAtoi(row.Find(`td[data-stat="4"]`).Text())
			ls.Total = mustAtoi(row.Find(`td[data-stat="T"]`).Text())
			lineScores = append(lineScores, ls)
		}
	})

	// 4. Print the results
	if len(lineScores) > 0 {
		log.Printf("✅ Success! Found %d line scores for game %s:", len(lineScores), gameID)
		for _, ls := range lineScores {
			log.Printf("%+v\n", ls)
		}
	} else {
		log.Println("❌ Table was found, but failed to parse any rows.")
	}
}

func main() {
	gameID := "202406170BOS"
	gameURL := fmt.Sprintf("https://www.basketball-reference.com/boxscores/%s.html", gameID)
	log.Printf("--- Scraping Line Score for Game: %s ---", gameID)
	log.Printf("--- URL: %s ---", gameURL)
	scrapeAndPrintLineScore(gameURL, gameID)
}