package main

import (
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
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

func TestParseEndpointMix(t *testing.T) {
	buckets, err := parseEndpointMix("playeradvancedstats:40,playertotals:30,games:30")
	if err != nil {
		t.Fatalf("parseEndpointMix error: %v", err)
	}
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}
	if buckets[0].endpoint != "/api/playeradvancedstats" || buckets[0].weight != 40 {
		t.Errorf("bucket 0 = %+v", buckets[0])
	}
	if buckets[1].endpoint != "/api/playertotals" || buckets[1].weight != 30 {
		t.Errorf("bucket 1 = %+v", buckets[1])
	}
	if buckets[2].endpoint != "/api/games" || buckets[2].weight != 30 {
		t.Errorf("bucket 2 = %+v", buckets[2])
	}
}

func TestGenerateRequestURLVariesParams(t *testing.T) {
	baseURL, _ := url.Parse("https://nba.turbo-data.com/api/playeradvancedstats")
	generatedURLs := make(map[string]bool)

	for i := 0; i < 50; i++ {
		rng := rand.New(rand.NewSource(int64(i + 100)))
		u := GenerateRequestURL(RequestOptions{
			BaseURL:    *baseURL,
			Page:       0,
			VaryParams: true,
			Complexity: "high",
			CacheBust:  false,
			RequestID:  i,
			WorkerID:   1,
			RNG:        rng,
		})
		generatedURLs[u] = true
		parsed, err := url.Parse(u)
		if err != nil {
			t.Fatalf("invalid url generated: %s", u)
		}
		q := parsed.Query()
		if q.Get("sortBy") == "" {
			t.Errorf("expected sortBy to be populated, got URL: %s", u)
		}
		if q.Get("pageSize") == "" {
			t.Errorf("expected pageSize to be populated, got URL: %s", u)
		}
	}

	if len(generatedURLs) < 40 {
		t.Errorf("expected high URL diversity (>40 distinct URLs for 50 requests), got %d", len(generatedURLs))
	}
}

func TestGenerateRequestURLCacheBust(t *testing.T) {
	baseURL, _ := url.Parse("https://nba.turbo-data.com/api/playeradvancedstats?page=1&pageSize=40")
	rng := rand.New(rand.NewSource(42))

	u1 := GenerateRequestURL(RequestOptions{
		BaseURL:   *baseURL,
		Page:      1,
		CacheBust: true,
		RequestID: 1,
		WorkerID:  1,
		RNG:       rng,
	})

	u2 := GenerateRequestURL(RequestOptions{
		BaseURL:   *baseURL,
		Page:      1,
		CacheBust: true,
		RequestID: 2,
		WorkerID:  1,
		RNG:       rng,
	})

	if u1 == u2 {
		t.Errorf("expected distinct URLs with cacheBust=true, got identical: %s", u1)
	}

	p1, _ := url.Parse(u1)
	if p1.Query().Get("_cb") == "" {
		t.Errorf("expected _cb parameter in URL %s", u1)
	}
}

func TestExtractCacheStatus(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		expected CacheStatus
	}{
		{
			name:     "Cloudflare HIT",
			headers:  http.Header{"Cf-Cache-Status": []string{"HIT"}},
			expected: CacheHit,
		},
		{
			name:     "Cloudflare MISS",
			headers:  http.Header{"Cf-Cache-Status": []string{"MISS"}},
			expected: CacheMiss,
		},
		{
			name:     "Cloudflare DYNAMIC",
			headers:  http.Header{"Cf-Cache-Status": []string{"DYNAMIC"}},
			expected: CacheMiss,
		},
		{
			name:     "X-Cache HIT",
			headers:  http.Header{"X-Cache": []string{"HIT from proxy"}},
			expected: CacheHit,
		},
		{
			name:     "Age header > 0",
			headers:  http.Header{"Age": []string{"120"}},
			expected: CacheHit,
		},
		{
			name:     "Age header 0 with no cache headers",
			headers:  http.Header{"Age": []string{"0"}},
			expected: CacheUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ExtractCacheStatus(tt.headers)
			if actual != tt.expected {
				t.Errorf("ExtractCacheStatus() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestCalculatePercentiles(t *testing.T) {
	durations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	stats := CalculatePercentiles(durations)
	if stats.Count != 10 {
		t.Errorf("Count = %d, want 10", stats.Count)
	}
	if stats.Min != 10*time.Millisecond {
		t.Errorf("Min = %v, want 10ms", stats.Min)
	}
	if stats.Max != 100*time.Millisecond {
		t.Errorf("Max = %v, want 100ms", stats.Max)
	}
	if stats.Average != 55*time.Millisecond {
		t.Errorf("Average = %v, want 55ms", stats.Average)
	}
	if stats.P50 != 55*time.Millisecond {
		t.Errorf("P50 = %v, want 55ms", stats.P50)
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
