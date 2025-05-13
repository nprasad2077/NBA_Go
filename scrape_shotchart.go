//go:build ignore
// +build ignore

package main

import (
    "bytes"
    "encoding/csv"
    "flag"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"

    "github.com/PuerkitoBio/goquery"
    "golang.org/x/net/html"
)

func main() {
    playerID    := flag.String("playerid", "", "Basketball-Reference player ID (e.g. derozde01)")
    playerName  := flag.String("playername", "", "Player name for file naming (e.g. \"DeMar DeRozan\")")
    startSeason := flag.Int("start", 0, "Recent start season (e.g. 2024)")
    endSeason   := flag.Int("end",   0, "Rookie/end season (e.g. 2021)")
    outDir      := flag.String("outdir", "", "If set, write CSV files into this directory; otherwise stdout")
    flag.Parse()

    if *playerID == "" || *playerName == "" || *startSeason == 0 || *endSeason == 0 {
        fmt.Fprintln(os.Stderr, "❌ must provide -playerid, -playername, -start and -end")
        os.Exit(1)
    }
    if *startSeason < *endSeason {
        fmt.Fprintln(os.Stderr, "❌ start must be >= end")
        os.Exit(1)
    }

    for season := *startSeason; season >= *endSeason; season-- {
        url := fmt.Sprintf(
            "https://www.basketball-reference.com/players/%s/%s/shooting/%d",
            (*playerID)[:1], *playerID, season,
        )
        fmt.Println("Fetching:", url)

        // Build a real-browser request
        client := &http.Client{}
        req, err := http.NewRequest("GET", url, nil)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Request creation error for season %d: %v\n", season, err)
            continue
        }
        req.Header.Set("User-Agent",
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "+
                "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36",
        )

        resp, err := client.Do(req)
        if err != nil {
            fmt.Fprintf(os.Stderr, "HTTP error for %d: %v\n", season, err)
            continue
        }
        bodyBytes, err := io.ReadAll(resp.Body)
        resp.Body.Close()
        if err != nil {
            fmt.Fprintf(os.Stderr, "Read error for %d: %v\n", season, err)
            continue
        }

        // Parse the full HTML to find the commented-out shot-chart block
        shotHTML := extractCommentedShotChart(bodyBytes)
        if shotHTML == "" {
            fmt.Fprintf(os.Stderr, "❌ no shot‐chart comment found for %d\n", season)
            continue
        }

        // Now parse that snippet
        doc, err := goquery.NewDocumentFromReader(strings.NewReader(shotHTML))
        if err != nil {
            fmt.Fprintf(os.Stderr, "Parse error for %d: %v\n", season, err)
            continue
        }

        // grab the shot-wrapper inside the commented HTML
        wrapper := doc.Find("div#div_shot-chart div#shot-wrapper")
        if wrapper.Length() == 0 {
            fmt.Fprintf(os.Stderr, "❌ no shot‐wrapper found in comment for %d\n", season)
            continue
        }

        shots := wrapper.Find("div.tooltip.make, div.tooltip.miss")
        var rows [][]string
        shots.Each(func(_ int, s *goquery.Selection) {
            style, _ := s.Attr("style")
            tip,   _ := s.Attr("tip")

            // parse top/left
            parts := strings.Split(style, ";")
            topVal := parsePx(parts[0])
            leftVal:= parsePx(parts[1])

            tipParts := strings.Split(tip, "<br>")
            // date, team code, opponent
            dateSegs := strings.SplitN(tipParts[0], ", ", 3)
            date := dateSegs[0] + "," + dateSegs[1]
            team := ""
            if len(dateSegs) == 3 {
                team = dateSegs[2]
            }
            opponent := ""
            if parts := strings.SplitN(tipParts[0], " at ", 2); len(parts) == 2 {
                opponent = parts[1]
            } else if parts := strings.SplitN(tipParts[0], " vs ", 2); len(parts) == 2 {
                opponent = parts[1]
            }

            // quarter & time
            qt := strings.SplitN(tipParts[1], ",", 2)
            quarter := qt[0]
            timeRem := strings.Fields(qt[1])[0]

            // result, shot type, distance
            rt := strings.Fields(tipParts[2])
            result := (rt[0] == "Made")
            shotType := rt[1]
            distance := parseInt(rt[len(rt)-2])

            // scores
            last := strings.Fields(tipParts[3])
            sc := strings.Split(last[len(last)-1], "-")
            teamScore     := parseInt(sc[0])
            opponentScore := parseInt(sc[1])
            lead := teamScore > opponentScore

            rows = append(rows, []string{
                *playerName,
                fmt.Sprint(topVal),
                fmt.Sprint(leftVal),
                date,
                quarter,
                timeRem,
                fmt.Sprint(result),
                shotType,
                fmt.Sprint(distance),
                fmt.Sprint(lead),
                fmt.Sprint(teamScore),
                fmt.Sprint(opponentScore),
                opponent,
                team,
                fmt.Sprint(season),
                *playerID,
            })
        })

        headers := []string{
            "playerName","top","left","date","qtr","timeRemaining",
            "result","shotType","distanceFt","lead",
            "teamScore","opponentTeamScore","opponent","team","season","playerId",
        }

        var w *csv.Writer
        if *outDir != "" {
            os.MkdirAll(*outDir, 0755)
            fname := filepath.Join(*outDir,
                fmt.Sprintf("%s_%d_shotchart.csv", *playerID, season),
            )
            f, err := os.Create(fname)
            if err != nil {
                fmt.Fprintf(os.Stderr, "Could not create %s: %v\n", fname, err)
                continue
            }
            defer f.Close()
            w = csv.NewWriter(f)
            fmt.Printf("→ Writing %d rows to %s\n", len(rows), fname)
        } else {
            w = csv.NewWriter(os.Stdout)
        }

        w.Write(headers)
        for _, r := range rows {
            w.Write(r)
        }
        w.Flush()
        if err := w.Error(); err != nil {
            fmt.Fprintf(os.Stderr, "CSV error: %v\n", err)
        }

        fmt.Printf("\nFirst 5 rows for %d:\n", season)
        for i := 0; i < len(rows) && i < 5; i++ {
            fmt.Println(rows[i])
        }
        fmt.Printf("✅ Season %d: %d total shots\n\n", season, len(rows))
    }
}

// extractCommentedShotChart scans the HTML for a comment node containing
// the div#div_shot-chart block, and returns its inner HTML.
func extractCommentedShotChart(htmlBytes []byte) string {
    root, err := html.Parse(bytes.NewReader(htmlBytes))
    if err != nil {
        return ""
    }
    var found string
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.CommentNode && strings.Contains(n.Data, `id="div_shot-chart"`) {
            found = n.Data
            return
        }
        for c := n.FirstChild; c != nil && found == ""; c = c.NextSibling {
            f(c)
        }
    }
    f(root)
    return found
}

func parsePx(s string) int {
    parts := strings.Split(s, ":")
    return parseInt(strings.TrimSuffix(parts[1], "px"))
}

func parseInt(s string) int {
    v, _ := strconv.Atoi(strings.TrimSpace(s))
    return v
}