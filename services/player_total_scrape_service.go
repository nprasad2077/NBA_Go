// File: NBA_Go/services/player_total_scrape_service.go

package services

import (
    "bytes"
    "fmt"
    "io"
    "log"
    "net/http"
    "strings"

    "github.com/PuerkitoBio/goquery"
    "github.com/nprasad2077/NBA_Go/models"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

const (
    regularURLFmt = "https://www.basketball-reference.com/leagues/NBA_%d_totals.html"
    playoffURLFmt = "https://www.basketball-reference.com/playoffs/NBA_%d_totals.html"
)

// urlForSeason chooses regular vs. playoff URL.
func urlForSeason(season int, isPlayoff bool) string {
    if isPlayoff {
        return fmt.Sprintf(playoffURLFmt, season)
    }
    return fmt.Sprintf(regularURLFmt, season)
}

// FetchAndStorePlayerTotalScrapedStats scrapes BR totals (regular or playoffs)
// and upserts into PlayerTotalStat.
func FetchAndStorePlayerTotalScrapedStats(db *gorm.DB, season int, isPlayoff bool) error {
    url := urlForSeason(season, isPlayoff)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return err 
    }

    doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
    if err != nil {
        return err
    }

    table := doc.Find("table#totals_stats")
    if table.Length() == 0 {
        return fmt.Errorf("could not find table#totals_stats")
    }

    // table := doc.Find("table#totals_stats")
    // if table.Length() == 0 {
    //     commentSel := doc.
    //         Find("div#all_totals_stats").
    //         Contents().
    //         FilterFunction(func(i int, s *goquery.Selection) bool {
    //             return goquery.NodeName(s) == "#comment"
    //         })

    //     if commentSel.Length() == 0 {
    //         return fmt.Errorf("could not find table#totals_stats (even inside comment)")
    //     }

    //     commentedHTML := commentSel.Nodes[0].FirstChild.Data
    //     innerDoc, err := goquery.NewDocumentFromReader(strings.NewReader(commentedHTML))
    //     if err != nil {
    //         return fmt.Errorf("failed to parse commented totals_stats HTML: %w", err)
    //     }
    //     table = innerDoc.Find("table#totals_stats")
    //     if table.Length() == 0 {
    //         return fmt.Errorf("could not find table#totals_stats after un‐commenting")
    //     }
    // }

    // 1) collect the data-stat keys in header order
    var headers []string
    table.Find("thead tr th").Each(func(i int, th *goquery.Selection) {
        if stat, ok := th.Attr("data-stat"); ok && stat != "" {
            headers = append(headers, stat)
        }
    })
    // add the appended-player column
    headers = append(headers, "player-additional")

    // 2) iterate rows
    table.Find("tbody tr").Each(func(_ int, tr *goquery.Selection) {
        if cl, _ := tr.Attr("class"); strings.Contains(cl, "thead") {
            return // skip header rows
        }

        cells := tr.Find("th, td")
        data := make(map[string]string, len(headers))
        var playerID string

        cells.Each(func(i int, cell *goquery.Selection) {
            text := strings.TrimSpace(cell.Text())
            key := headers[i]
            data[key] = text
            if id, ok := cell.Attr("data-append-csv"); ok {
                playerID = id
            }
        })
        if playerID == "" {
            return
        }
        data["player-additional"] = playerID

    // 3a) pick the right “ExternalID” key (playoffs use “rk”; season uses “ranker”)
    extID := mustAtoi(data["rk"])
    if extID == 0 {
        extID = mustAtoi(data["ranker"])
    }

    // 3b) pick the right “PlayerName” key (playoffs use “player”; season uses “name_display”)
    playerName := data["player"]
    if playerName == "" {
        playerName = data["name_display"]
    }

    // 3c) pick the right “Team” key (playoffs use “team_id”; season uses “team_name_abbr”)
    teamID := data["team_id"]
    if teamID == "" {
        teamID = data["team_name_abbr"]
    }

    // pick “games” → fallback to “g” if empty
    g := mustAtoi(data["games"])
    if g == 0 {
        g = mustAtoi(data["g"])
    }

    // pick “games_started” → fallback to “gs” if empty
    gs := mustAtoi(data["games_started"])
    if gs == 0 {
        gs = mustAtoi(data["gs"])
    }

    stat := models.PlayerTotalStat{
        ExternalID:    extID,
        PlayerID:      playerID,
        PlayerName:    playerName,
        Position:      data["pos"],
        Age:           mustAtoi(data["age"]),
        Games:         g,
        GamesStarted:  gs,
        MinutesPG:     mustParseFloat(data["mp"]),
        FieldGoals:    mustAtoi(data["fg"]),
        FieldAttempts: mustAtoi(data["fga"]),
        FieldPercent:  mustParseFloat(data["fg_pct"]),
        ThreeFG:       mustAtoi(data["fg3"]),
        ThreeAttempts: mustAtoi(data["fg3a"]),
        ThreePercent:  mustParseFloat(data["fg3_pct"]),
        TwoFG:         mustAtoi(data["fg2"]),
        TwoAttempts:   mustAtoi(data["fg2a"]),
        TwoPercent:    mustParseFloat(data["fg2_pct"]),
        EffectFGPercent: mustParseFloat(data["efg_pct"]),
        FT:            mustAtoi(data["ft"]),
        FTAttempts:    mustAtoi(data["fta"]),
        FTPercent:     mustParseFloat(data["ft_pct"]),
        OffensiveRB:   mustAtoi(data["orb"]),
        DefensiveRB:   mustAtoi(data["drb"]),
        TotalRB:       mustAtoi(data["trb"]),
        Assists:       mustAtoi(data["ast"]),
        Steals:        mustAtoi(data["stl"]),
        Blocks:        mustAtoi(data["blk"]),
        Turnovers:     mustAtoi(data["tov"]),
        PersonalFouls: mustAtoi(data["pf"]),
        Points:        mustAtoi(data["pts"]),
        Team:          teamID,
        Season:        season,
        IsPlayoff:     isPlayoff,
    }

        // 4) upsert on (player_id, season, team, is_playoff)
        if err := db.Clauses(clause.OnConflict{
            Columns: []clause.Column{
                {Name: "player_id"},
                {Name: "season"},
                {Name: "team"},
                {Name: "is_playoff"},
            },
            DoUpdates: clause.AssignmentColumns([]string{
                "external_id", "player_name", "position", "age",
                "games", "games_started", "minutes_pg",
                "field_goals", "field_attempts", "field_percent",
                "three_fg", "three_attempts", "three_percent",
                "two_fg", "two_attempts", "two_percent",
                "effect_fg_percent",
                "ft", "ft_attempts", "ft_percent",
                "offensive_rb", "defensive_rb", "total_rb",
                "assists", "steals", "blocks", "turnovers",
                "personal_fouls", "points",
            }),
        }).Create(&stat).Error; err != nil {
            log.Printf("Failed upsert for %s: %v", stat.PlayerID, err)
        }
    })

    return nil
}