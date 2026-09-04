package main

import (
	"net/url"
	"reflect"
	"testing"
)

func TestParsePageMixDistributesRangeWeights(t *testing.T) {
	buckets, err := parsePageMix("1-3:60,4-10:30,11-20:10")
	if err != nil {
		t.Fatalf("parsePageMix returned an error: %v", err)
	}

	if len(buckets) != 20 {
		t.Fatalf("expected 20 page buckets, got %d", len(buckets))
	}

	for page := 1; page <= 20; page++ {
		bucket := buckets[page-1]
		if bucket.page != page {
			t.Errorf("bucket %d has page %d", page-1, bucket.page)
		}

		expectedWeight := 1.0
		if page <= 3 {
			expectedWeight = 20
		} else if page <= 10 {
			expectedWeight = 30.0 / 7
		}
		if bucket.weight != expectedWeight {
			t.Errorf("page %d has weight %v, want %v", page, bucket.weight, expectedWeight)
		}
	}
}

func TestParsePageMixRejectsInvalidSpecifications(t *testing.T) {
	invalidSpecs := []string{
		"1-3",
		"0:1",
		"3-1:1",
		"1-3:0",
		"letters:1",
		"1:1,1:2",
	}

	for _, spec := range invalidSpecs {
		t.Run(spec, func(t *testing.T) {
			if _, err := parsePageMix(spec); err == nil {
				t.Fatalf("parsePageMix(%q) returned nil error", spec)
			}
		})
	}
}

func TestSelectPagesUsesConfiguredWeights(t *testing.T) {
	buckets, err := parsePageMix("1-3:60,4-10:30,11-20:10")
	if err != nil {
		t.Fatalf("parsePageMix returned an error: %v", err)
	}

	pages := selectPages(buckets, 10000, 42)
	if len(pages) != 10000 {
		t.Fatalf("expected 10000 selected pages, got %d", len(pages))
	}

	counts := make(map[int]int)
	for _, page := range pages {
		counts[page]++
	}

	assertCountNear(t, "pages 1-3", counts[1]+counts[2]+counts[3], 6000, 150)
	assertCountNear(t, "pages 4-10", sumPageCounts(counts, 4, 10), 3000, 150)
	assertCountNear(t, "pages 11-20", sumPageCounts(counts, 11, 20), 1000, 100)
}

func TestSelectPagesIsReproducibleWithSeed(t *testing.T) {
	buckets, err := parsePageMix("1:3,2:1")
	if err != nil {
		t.Fatalf("parsePageMix returned an error: %v", err)
	}

	first := selectPages(buckets, 20, 42)
	second := selectPages(buckets, 20, 42)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("page selections differ for the same seed: %v != %v", first, second)
	}
}

func TestURLForPageReplacesOnlyPageParameter(t *testing.T) {
	base, err := url.Parse("https://example.test/api/playertotals?page=1&pageSize=50&season=2025")
	if err != nil {
		t.Fatalf("url.Parse returned an error: %v", err)
	}

	requestURL, err := url.Parse(urlForPage(*base, 7))
	if err != nil {
		t.Fatalf("url.Parse returned an error: %v", err)
	}

	query := requestURL.Query()
	if query.Get("page") != "7" {
		t.Errorf("page query parameter = %q, want 7", query.Get("page"))
	}
	if query.Get("pageSize") != "50" {
		t.Errorf("pageSize query parameter = %q, want 50", query.Get("pageSize"))
	}
	if query.Get("season") != "2025" {
		t.Errorf("season query parameter = %q, want 2025", query.Get("season"))
	}
}

func assertCountNear(t *testing.T, label string, actual, expected, tolerance int) {
	t.Helper()
	if actual < expected-tolerance || actual > expected+tolerance {
		t.Errorf("%s count = %d, want %d +/- %d", label, actual, expected, tolerance)
	}
}

func sumPageCounts(counts map[int]int, firstPage, lastPage int) int {
	total := 0
	for page := firstPage; page <= lastPage; page++ {
		total += counts[page]
	}
	return total
}
