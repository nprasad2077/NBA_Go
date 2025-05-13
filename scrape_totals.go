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
    "strings"

    "github.com/PuerkitoBio/goquery"
)

func main() {
    season := flag.Int("season", 0, "NBA season year (e.g. 2024)")
    outPath := flag.String("out", "", "output CSV file path (default stdout)")
    flag.Parse()

    if *season == 0 {
        fmt.Fprintln(os.Stderr, "❌ must provide -season")
        os.Exit(1)
    }

    url := fmt.Sprintf("https://www.basketball-reference.com/playoffs/NBA_%d_totals.html", *season)
    fmt.Println("Fetching:", url)

    // pretend to be a real browser
    client := &http.Client{}
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        fmt.Fprintln(os.Stderr, "Request error:", err)
        os.Exit(1)
    }
    req.Header.Set("User-Agent",
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "+
            "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36",
    )
    resp, err := client.Do(req)
    if err != nil {
        fmt.Fprintln(os.Stderr, "HTTP error:", err)
        os.Exit(1)
    }
    body, err := io.ReadAll(resp.Body)
    resp.Body.Close()
    if err != nil {
        fmt.Fprintln(os.Stderr, "Read error:", err)
        os.Exit(1)
    }

    doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
    if err != nil {
        fmt.Fprintln(os.Stderr, "Parse error:", err)
        os.Exit(1)
    }

    table := doc.Find("table#totals_stats")
    if table.Length() == 0 {
        fmt.Fprintln(os.Stderr, "❌ could not find table with id 'totals_stats'")
        os.Exit(1)
    }

    // Extract headers
    var headers []string
    table.Find("thead tr th").Each(func(_ int, th *goquery.Selection) {
        if stat, exists := th.Attr("data-stat"); exists && stat != "" {
            headers = append(headers, stat)
        }
    })
    // ensure player-additional
    headers = append(headers, "player-additional")

    // Extract rows
    var rows [][]string
    table.Find("tbody tr").Each(func(_ int, tr *goquery.Selection) {
        if cl, _ := tr.Attr("class"); strings.Contains(cl, "thead") {
            return
        }
        var row []string
        var playerID string
        tr.Find("th, td").Each(func(_ int, cell *goquery.Selection) {
            row = append(row, strings.TrimSpace(cell.Text()))
            if id, ok := cell.Attr("data-append-csv"); ok {
                playerID = id
            }
        })
        if playerID != "" {
            row = append(row, playerID)
            rows = append(rows, row)
        }
    })

    // open writer
    var w *csv.Writer
    if *outPath != "" {
        f, err := os.Create(*outPath)
        if err != nil {
            fmt.Fprintln(os.Stderr, "Could not create file:", err)
            os.Exit(1)
        }
        defer f.Close()
        w = csv.NewWriter(f)
        fmt.Printf("→ Writing %d rows to %s\n", len(rows), *outPath)
    } else {
        w = csv.NewWriter(os.Stdout)
    }

    // write CSV
    w.Write(headers)
    for _, r := range rows {
        w.Write(r)
    }
    w.Flush()
    if err := w.Error(); err != nil {
        fmt.Fprintln(os.Stderr, "CSV write error:", err)
        os.Exit(1)
    }
    fmt.Println("First 5 rows:")
    for i := 0; i < len(rows) && i < 5; i++ {
        fmt.Println(rows[i])
    }
    fmt.Printf("✅ Wrote %d rows for season %d\n", len(rows), *season)
}