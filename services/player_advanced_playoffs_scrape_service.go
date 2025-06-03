// File: NBA_Go/services/player_advanced_scrape_service.go

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
    advancedURLFmt        = "https://www.basketball-reference.com/leagues/NBA_%d_advanced.html"
    advancedPlayoffURLFmt = "https://www.basketball-reference.com/playoffs/NBA_%d_advanced.html"
)

// urlForAdvSeason picks the correct URL based on isPlayoff.
func urlForAdvSeason(season int, isPlayoff bool) string {
    if isPlayoff {
        return fmt.Sprintf(advancedPlayoffURLFmt, season)
    }
    return fmt.Sprintf(advancedURLFmt, season)
}

// FetchAndStorePlayerAdvancedScrapedStats scrapes the advanced table
// (regular or playoffs) and upserts into the PlayerAdvancedStat model.
func FetchAndStorePlayerAdvancedScrapedStats(db *gorm.DB, season int, isPlayoff bool) error {
    url := urlForAdvSeason(season, isPlayoff)
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

    htmlBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }

    doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
    if err != nil {
        return err
    }

    // pick the right table selector
    var table *goquery.Selection
    if isPlayoff {
        table = doc.Find("#div_advanced_stats table#advanced_stats")
    } else {
        table = doc.Find("#div_advanced table#advanced")
    }
    if table.Length() == 0 {
        return fmt.Errorf("could not find advanced stats table")
    }

    // 1. collect the data-stat keys in header order
    var headers []string
    table.Find("thead tr th").Each(func(i int, th *goquery.Selection) {
        if stat, ok := th.Attr("data-stat"); ok && stat != "" {
            headers = append(headers, stat)
        }
    })
    // add our appended-player column
    headers = append(headers, "player-additional")

    // 2. iterate each row
    table.Find("tbody tr").Each(func(_ int, tr *goquery.Selection) {
        if tr.HasClass("thead") {
            return // skip repeated headers
        }
        cells := tr.Find("th, td")
        data := make(map[string]string, len(headers))
        var playerID string

        cells.Each(func(i int, cell *goquery.Selection) {
            key := headers[i]
            data[key] = strings.TrimSpace(cell.Text())
            if id, ok := cell.Attr("data-append-csv"); ok {
                playerID = id
            }
        })
        if playerID == "" {
            return // not a data row
        }
        data["player-additional"] = playerID

        // 4) pick the correct keys for ExternalID, PlayerName, Team, depending on season vs. playoff
        //    - playoffs tables still use "rk", "player", "team_id"
        //    - regular‐season advanced uses "ranker", "name_display", "team_name_abbr"
        extID := mustAtoi(data["rk"])
        if extID == 0 {
            extID = mustAtoi(data["ranker"])
        }

        playerName := data["player"]
        if playerName == "" {
            playerName = data["name_display"]
        }

        teamID := data["team_id"]
        if teamID == "" {
            teamID = data["team_name_abbr"]
        }

        // 5) map into your GORM model
        stat := models.PlayerAdvancedStat{
            ExternalID:         extID,
            PlayerID:           playerID,
            PlayerName:         playerName,
            Position:           data["pos"],
            Age:                mustAtoi(data["age"]),
            Games:              mustAtoi(data["g"]),
            GamesStarted:       mustAtoi(data["games_started"]),
            MinutesPlayed:      mustAtoi(data["mp"]),
            PER:                mustParseFloat(data["per"]),
            TSPercent:          mustParseFloat(data["ts_pct"]),
            ThreePAR:           mustParseFloat(data["fg3a_per_fga_pct"]),
            FTR:                mustParseFloat(data["fta_per_fga_pct"]),
            OffensiveRBPercent: mustParseFloat(data["orb_pct"]),
            DefensiveRBPercent: mustParseFloat(data["drb_pct"]),
            TotalRBPercent:     mustParseFloat(data["trb_pct"]),
            AssistPercent:      mustParseFloat(data["ast_pct"]),
            StealPercent:       mustParseFloat(data["stl_pct"]),
            BlockPercent:       mustParseFloat(data["blk_pct"]),
            TurnoverPercent:    mustParseFloat(data["tov_pct"]),
            UsagePercent:       mustParseFloat(data["usg_pct"]),
            OffensiveWS:        mustParseFloat(data["ows"]),
            DefensiveWS:        mustParseFloat(data["dws"]),
            WinShares:          mustParseFloat(data["ws"]),
            WinSharesPer:       mustParseFloat(data["ws_per_48"]),
            OffensiveBox:       mustParseFloat(data["obpm"]),
            DefensiveBox:       mustParseFloat(data["dbpm"]),
            Box:                mustParseFloat(data["bpm"]),
            VORP:               mustParseFloat(data["vorp"]),
            Team:               teamID,
            Season:             season,
            IsPlayoff:          isPlayoff,
        }

        // 6) upsert on (player_id, season, team, is_playoff)
        if err := db.Clauses(clause.OnConflict{
            Columns: []clause.Column{
                {Name: "player_id"},
                {Name: "season"},
                {Name: "team"},
                {Name: "is_playoff"},
            },
            DoUpdates: clause.AssignmentColumns([]string{
                "external_id", "player_name", "position", "age", "games", "games_started", 
                "minutes_played",
                "per", "ts_percent", "three_par", "ftr",
                "offensive_rb_percent", "defensive_rb_percent", "total_rb_percent",
                "assist_percent", "steal_percent", "block_percent", "turnover_percent",
                "usage_percent", "offensive_ws", "defensive_ws", "win_shares",
                "win_shares_per", "offensive_box", "defensive_box", "box", "vorp",
            }),
        }).Create(&stat).Error; err != nil {
            log.Printf("Failed upsert advanced for %s: %v", stat.PlayerID, err)
        }
    })

    return nil
}

//         // 3. map into your model
        // stat := models.PlayerAdvancedStat{
//             ExternalID:         mustAtoi(data["rk"]),
//             PlayerID:           playerID,
//             PlayerName:         data["player"],
//             Position:           data["pos"],
//             Age:                mustAtoi(data["age"]),
//             Games:              mustAtoi(data["g"]),
//             MinutesPlayed:      mustAtoi(data["mp"]),
//             PER:                mustParseFloat(data["per"]),
//             TSPercent:          mustParseFloat(data["ts_pct"]),
//             ThreePAR:           mustParseFloat(data["three_par"]),
//             FTR:                mustParseFloat(data["ftr"]),
//             OffensiveRBPercent: mustParseFloat(data["orb_pct"]),
//             DefensiveRBPercent: mustParseFloat(data["drb_pct"]),
//             TotalRBPercent:     mustParseFloat(data["trb_pct"]),
//             AssistPercent:      mustParseFloat(data["ast_pct"]),
//             StealPercent:       mustParseFloat(data["stl_pct"]),
//             BlockPercent:       mustParseFloat(data["blk_pct"]),
//             TurnoverPercent:    mustParseFloat(data["tov_pct"]),
//             UsagePercent:       mustParseFloat(data["usg_pct"]),
//             OffensiveWS:        mustParseFloat(data["ows"]),
//             DefensiveWS:        mustParseFloat(data["dws"]),
//             WinShares:          mustParseFloat(data["ws"]),
//             WinSharesPer:       mustParseFloat(data["ws_per_48"]),
//             OffensiveBox:       mustParseFloat(data["obpm"]),
//             DefensiveBox:       mustParseFloat(data["dbpm"]),
//             Box:                mustParseFloat(data["bpm"]),
//             VORP:               mustParseFloat(data["vorp"]),
//             Team:               data["team_id"],
//             Season:             season,
//             IsPlayoff:          isPlayoff,
//         }

//         // 4. upsert
//         if err := db.Clauses(clause.OnConflict{
//             Columns: []clause.Column{
//                 {Name: "player_id"},
//                 {Name: "season"},
//                 {Name: "team"},
//                 {Name: "is_playoff"},
//             },
//             DoUpdates: clause.AssignmentColumns([]string{
//                 "external_id", "player_name", "position", "age", "games", "minutes_played",
//                 "per", "ts_percent", "three_par", "ftr",
//                 "offensive_rb_percent", "defensive_rb_percent", "total_rb_percent",
//                 "assist_percent", "steal_percent", "block_percent", "turnover_percent",
//                 "usage_percent", "offensive_ws", "defensive_ws", "win_shares",
//                 "win_shares_per", "offensive_box", "defensive_box", "box", "vorp",
//             }),
//         }).Create(&stat).Error; err != nil {
//             log.Printf("Failed upsert advanced for %s: %v", stat.PlayerID, err)
//         }
//     })

//     return nil
// }