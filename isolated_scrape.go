//go:build ignore
// +build ignore

package main

import (
    "encoding/csv"
    "flag"
    "fmt"
    "net/http"
    "os"
    // "strconv"
    "strings"

    "github.com/PuerkitoBio/goquery"
)

func main() {
    season := flag.Int("season", 0, "NBA season year (e.g. 2024)")
    outPath := flag.String("out", "", "output CSV file path (default: stdout)")
    flag.Parse()

    if *season == 0 {
        fmt.Fprintln(os.Stderr, "❌ must provide -season, e.g. -season=2024")
        os.Exit(1)
    }

    url := fmt.Sprintf("https://www.basketball-reference.com/playoffs/NBA_%d_advanced.html", *season)
    fmt.Println("Fetching:", url)
    resp, err := http.Get(url)
    if err != nil {
        fmt.Fprintln(os.Stderr, "HTTP error:", err)
        os.Exit(1)
    }
    defer resp.Body.Close()

    doc, err := goquery.NewDocumentFromReader(resp.Body)
    if err != nil {
        fmt.Fprintln(os.Stderr, "Parse error:", err)
        os.Exit(1)
    }

    // --- Extract headers ---
    var headers []string
    doc.Find("#div_advanced_stats table#advanced_stats thead tr th").EachWithBreak(func(i int, s *goquery.Selection) bool {
        // grab first 29 columns, then we'll append our own PlayerID
        text := strings.TrimSpace(s.Text())
        if i < 29 {
            headers = append(headers, text)
            return true
        }
        return false
    })
    headers = append(headers, "Player-additional")

    // --- Extract data rows ---
    var rows [][]string
    doc.Find("#div_advanced_stats table#advanced_stats tbody tr").Each(func(_ int, row *goquery.Selection) {
        if row.HasClass("thead") { // skip repeated headers
            return
        }
        // rank in <th>
        vals := []string{strings.TrimSpace(row.Find("th").Text())}
        row.Find("td").Each(func(__ int, cell *goquery.Selection) {
            vals = append(vals, strings.TrimSpace(cell.Text()))
        })
        if id, ok := row.Find("td[data-append-csv]").Attr("data-append-csv"); ok {
            vals = append(vals, id)
            rows = append(rows, vals)
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
    } else {
        w = csv.NewWriter(os.Stdout)
    }
    // write header + rows
    w.Write(headers)
    for _, r := range rows {
        w.Write(r)
    }
    w.Flush()
    if err := w.Error(); err != nil {
        fmt.Fprintln(os.Stderr, "CSV write error:", err)
        os.Exit(1)
    }

    // also print first 5 records to console
    fmt.Println("\nFirst 5 rows:")
    for i := 0; i < len(rows) && i < 5; i++ {
        fmt.Println(rows[i])
    }
    fmt.Printf("\n✅ Wrote %d rows\n", len(rows))
}