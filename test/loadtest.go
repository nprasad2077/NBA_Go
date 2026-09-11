package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CacheStatus tracks edge/CDN cache outcome
type CacheStatus string

const (
	CacheHit     CacheStatus = "HIT"
	CacheMiss    CacheStatus = "MISS"
	CacheUnknown CacheStatus = "UNKNOWN"
)

// Response tracks the result of an individual HTTP request
type Response struct {
	StatusCode  int
	BodyLength  int
	Duration    time.Duration
	Error       error
	Page        int
	Endpoint    string
	CacheStatus CacheStatus
	URL         string
	Attempt     int
}

type pageBucket struct {
	page   int
	weight float64
}

type endpointBucket struct {
	endpoint string
	weight   float64
}

// PercentileStats holds distribution percentiles
type PercentileStats struct {
	Count   int
	Min     time.Duration
	P50     time.Duration
	P75     time.Duration
	P90     time.Duration
	P95     time.Duration
	P99     time.Duration
	Max     time.Duration
	Average time.Duration
}

// Static dataset pools for query permutations
var (
	teamAbbrs = []string{
		"ATL", "BOS", "BKN", "CHA", "CHI", "CLE", "DAL", "DEN", "DET", "GSW",
		"HOU", "IND", "LAC", "LAL", "MEM", "MIA", "MIL", "MIN", "NOP", "NYK",
		"OKC", "ORL", "PHI", "PHX", "POR", "SAC", "SAS", "TOR", "UTA", "WAS",
	}

	seasons = []int{2015, 2016, 2017, 2018, 2019, 2020, 2021, 2022, 2023, 2024, 2025}

	advancedSortFields = []string{
		"winShares", "vorp", "per", "tsPercent", "threePAR", "ftr",
		"offensiveRBPercent", "defensiveRBPercent", "totalRBPercent",
		"assistPercent", "stealPercent", "blockPercent", "turnoverPercent",
		"usagePercent", "offensiveWS", "defensiveWS", "winSharesPer",
		"offensiveBox", "defensiveBox", "box", "games", "minutesPlayed", "age",
	}

	totalsSortFields = []string{
		"points", "assists", "rebounds", "steals", "blocks", "games",
		"minutesPlayed", "turnovers", "fieldGoals", "threePoints", "freeThrows",
	}

	gameAssociations = []string{
		"lineScores", "playerGameBasicStats", "playerGameAdvStats",
		"teamGameBasicStats", "teamGameAdvStats",
	}

	pageSizes = []int{10, 20, 25, 30, 40, 50}

	playerIDs = []string{
		"curryst01", "jamesle01", "antetgi01", "jokicni01", "doncilu01",
		"tatumja01", "embiijo01", "duranke01", "butleji01", "moranja01",
	}
)

func parsePageMix(spec string) ([]pageBucket, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}

	seenPages := make(map[int]bool)
	var buckets []pageBucket
	for _, rawEntry := range strings.Split(spec, ",") {
		entry := strings.TrimSpace(rawEntry)
		parts := strings.Split(entry, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid page mix entry %q: expected pages:weight", entry)
		}

		pageParts := strings.Split(strings.TrimSpace(parts[0]), "-")
		if len(pageParts) > 2 {
			return nil, fmt.Errorf("invalid page range %q", parts[0])
		}

		firstPage, err := strconv.Atoi(strings.TrimSpace(pageParts[0]))
		if err != nil || firstPage < 1 {
			return nil, fmt.Errorf("invalid first page %q", pageParts[0])
		}

		lastPage := firstPage
		if len(pageParts) == 2 {
			lastPage, err = strconv.Atoi(strings.TrimSpace(pageParts[1]))
			if err != nil || lastPage < firstPage {
				return nil, fmt.Errorf("invalid page range %q", parts[0])
			}
		}

		weight, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil || weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			return nil, fmt.Errorf("invalid weight %q", parts[1])
		}

		pageCount := lastPage - firstPage + 1
		weightPerPage := weight / float64(pageCount)
		for page := firstPage; page <= lastPage; page++ {
			if seenPages[page] {
				return nil, fmt.Errorf("page %d appears in overlapping ranges", page)
			}
			seenPages[page] = true
			buckets = append(buckets, pageBucket{page: page, weight: weightPerPage})
		}
	}

	return buckets, nil
}

func parseEndpointMix(spec string) ([]endpointBucket, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}

	var buckets []endpointBucket
	for _, rawEntry := range strings.Split(spec, ",") {
		entry := strings.TrimSpace(rawEntry)
		parts := strings.Split(entry, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid endpoint mix entry %q: expected endpoint:weight", entry)
		}
		endpoint := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(endpoint, "/") {
			endpoint = "/api/" + endpoint
		}

		weight, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil || weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			return nil, fmt.Errorf("invalid weight %q for endpoint %q", parts[1], endpoint)
		}
		buckets = append(buckets, endpointBucket{endpoint: endpoint, weight: weight})
	}
	return buckets, nil
}

func selectPages(buckets []pageBucket, count int, seed int64) []int {
	pages := make([]int, count)
	if len(buckets) == 0 || count == 0 {
		return pages
	}

	totalWeight := 0.0
	for _, bucket := range buckets {
		totalWeight += bucket.weight
	}

	rng := rand.New(rand.NewSource(seed))
	for i := range pages {
		target := rng.Float64() * totalWeight
		cumulativeWeight := 0.0
		for _, bucket := range buckets {
			cumulativeWeight += bucket.weight
			if target < cumulativeWeight {
				pages[i] = bucket.page
				break
			}
		}
	}
	return pages
}

func selectEndpoint(buckets []endpointBucket, rng *rand.Rand) string {
	if len(buckets) == 0 {
		return ""
	}
	totalWeight := 0.0
	for _, bucket := range buckets {
		totalWeight += bucket.weight
	}

	target := rng.Float64() * totalWeight
	cumulativeWeight := 0.0
	for _, bucket := range buckets {
		cumulativeWeight += bucket.weight
		if target < cumulativeWeight {
			return bucket.endpoint
		}
	}
	return buckets[0].endpoint
}

func urlForPage(base url.URL, page int) string {
	query := base.Query()
	query.Set("page", strconv.Itoa(page))
	base.RawQuery = query.Encode()
	return base.String()
}

// RequestOptions configures dynamic URL and request generation
type RequestOptions struct {
	BaseURL     url.URL
	Page        int
	VaryParams  bool
	Complexity  string
	CacheBust   bool
	EndpointMix []endpointBucket
	RequestID   int
	WorkerID    int
	RNG         *rand.Rand
}

// GenerateRequestURL builds a customized URL based on options and randomness
func GenerateRequestURL(opts RequestOptions) string {
	targetURL := opts.BaseURL

	// If endpoint mix is configured, choose target endpoint
	if len(opts.EndpointMix) > 0 {
		ep := selectEndpoint(opts.EndpointMix, opts.RNG)
		targetURL.Path = ep
	}

	query := targetURL.Query()

	if opts.Page > 0 {
		query.Set("page", strconv.Itoa(opts.Page))
	} else if opts.VaryParams {
		// Random page 1-20
		query.Set("page", strconv.Itoa(opts.RNG.Intn(20)+1))
	}

	path := strings.ToLower(targetURL.Path)

	if opts.VaryParams {
		// Randomize pageSize
		query.Set("pageSize", strconv.Itoa(pageSizes[opts.RNG.Intn(len(pageSizes))]))

		// Randomize ascending
		query.Set("ascending", strconv.FormatBool(opts.RNG.Intn(2) == 1))

		// Randomize season (70% chance to filter by season)
		if opts.RNG.Float64() < 0.70 {
			query.Set("season", strconv.Itoa(seasons[opts.RNG.Intn(len(seasons))]))
		}

		// Randomize team filter (50% chance)
		if opts.RNG.Float64() < 0.50 {
			query.Set("team", teamAbbrs[opts.RNG.Intn(len(teamAbbrs))])
		}

		// Randomize isPlayoff (30% chance)
		if opts.RNG.Float64() < 0.30 {
			query.Set("isPlayoff", strconv.FormatBool(opts.RNG.Intn(2) == 1))
		}

		// Endpoint specific parameters
		if strings.Contains(path, "playeradvanced") {
			query.Set("sortBy", advancedSortFields[opts.RNG.Intn(len(advancedSortFields))])
			if opts.Complexity == "high" && opts.RNG.Float64() < 0.30 {
				query.Set("playerId", playerIDs[opts.RNG.Intn(len(playerIDs))])
			}
		} else if strings.Contains(path, "playertotal") {
			query.Set("sortBy", totalsSortFields[opts.RNG.Intn(len(totalsSortFields))])
			if opts.Complexity == "high" && opts.RNG.Float64() < 0.30 {
				query.Set("playerId", playerIDs[opts.RNG.Intn(len(playerIDs))])
			}
		} else if strings.Contains(path, "game") {
			query.Set("sortBy", "date")
			if opts.Complexity == "high" {
				// Random subset of 1 to 4 preloaded associations to stress joins
				numAssoc := opts.RNG.Intn(3) + 1
				perm := opts.RNG.Perm(len(gameAssociations))
				var chosen []string
				for i := 0; i < numAssoc; i++ {
					chosen = append(chosen, gameAssociations[perm[i]])
				}
				query.Set("include", strings.Join(chosen, ","))
			}
		} else if strings.Contains(path, "shotchart") {
			if opts.RNG.Float64() < 0.60 {
				query.Set("playerId", playerIDs[opts.RNG.Intn(len(playerIDs))])
			}
		}
	}

	if opts.CacheBust {
		// Unique high-resolution nonce guaranteed to miss CDN / reverse-proxy cache
		nonce := fmt.Sprintf("%d_%d_%d", time.Now().UnixNano(), opts.WorkerID, opts.RequestID)
		query.Set("_cb", nonce)
	}

	targetURL.RawQuery = query.Encode()
	return targetURL.String()
}

// ExtractCacheStatus parses response headers to identify cache state
func ExtractCacheStatus(headers http.Header) CacheStatus {
	// 1. Cloudflare CF-Cache-Status
	if cf := strings.ToUpper(headers.Get("CF-Cache-Status")); cf != "" {
		switch cf {
		case "HIT":
			return CacheHit
		case "MISS", "EXPIRED", "DYNAMIC", "BYPASS", "REVALIDATED":
			return CacheMiss
		}
	}

	// 2. Fastly / Varnish / Nginx X-Cache or X-Cache-Status
	if xc := strings.ToUpper(headers.Get("X-Cache-Status")); xc != "" {
		if strings.Contains(xc, "HIT") {
			return CacheHit
		}
		if strings.Contains(xc, "MISS") {
			return CacheMiss
		}
	}
	if xc := strings.ToUpper(headers.Get("X-Cache")); xc != "" {
		if strings.Contains(xc, "HIT") {
			return CacheHit
		}
		if strings.Contains(xc, "MISS") {
			return CacheMiss
		}
	}

	// 3. Age header (standard HTTP caching: Age > 0 implies served from cache)
	if ageStr := headers.Get("Age"); ageStr != "" {
		if ageVal, err := strconv.Atoi(ageStr); err == nil && ageVal > 0 {
			return CacheHit
		}
	}

	return CacheUnknown
}

// CalculatePercentiles computes key percentile distributions from durations
func CalculatePercentiles(durations []time.Duration) PercentileStats {
	n := len(durations)
	if n == 0 {
		return PercentileStats{}
	}

	sorted := make([]time.Duration, n)
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}

	getPercentile := func(p float64) time.Duration {
		if n == 1 {
			return sorted[0]
		}
		rank := p * float64(n-1)
		idx := int(math.Floor(rank))
		if idx >= n-1 {
			return sorted[n-1]
		}
		frac := rank - float64(idx)
		return sorted[idx] + time.Duration(float64(sorted[idx+1]-sorted[idx])*frac)
	}

	return PercentileStats{
		Count:   n,
		Min:     sorted[0],
		P50:     getPercentile(0.50),
		P75:     getPercentile(0.75),
		P90:     getPercentile(0.90),
		P95:     getPercentile(0.95),
		P99:     getPercentile(0.99),
		Max:     sorted[n-1],
		Average: sum / time.Duration(n),
	}
}

func main() {
	// Command line flags
	numRequests := flag.Int("n", 100, "Number of requests to send")
	concurrency := flag.Int("c", 10, "Number of concurrent requests")
	endpoint := flag.String("url", "http://localhost:5000/api/playeradvancedstats", "API endpoint to test")
	logFile := flag.String("log", "loadtest.log", "Log file path")
	apiKey := flag.String("key", "", "API key for x-api-key header")
	benchmarkKey := flag.String("benchmarkKey", "", "Benchmark key for X-Benchmark-Key header (bypasses rate limiting)")
	pageMix := flag.String("pageMix", "", "Weighted page mix, for example 1-3:60,4-10:30,11-20:10")
	varyParams := flag.Bool("varyParams", false, "Randomize query parameters (season, team, sortBy, pageSize, etc.)")
	complexity := flag.String("complexity", "standard", "Query complexity level: standard, high")
	cacheBust := flag.Bool("cacheBust", false, "Append unique query nonce to guarantee 100% cold-cache misses")
	rotateIPs := flag.Bool("rotateIPs", false, "Rotate synthetic client IPs (X-Real-IP / X-Forwarded-For) to simulate distributed traffic")
	retryOnRateLimit := flag.Bool("retryOnRateLimit", false, "Retry requests that receive 429 Too Many Requests with backoff")
	endpointMix := flag.String("endpointMix", "", "Weighted multi-endpoint mix, for example playeradvancedstats:40,playertotals:30,games:30")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP request timeout per request")
	seed := flag.Int64("seed", -1, "Random seed (-1 uses current timestamp)")
	flag.Parse()

	if *numRequests <= 0 {
		log.Fatal("Number of requests must be greater than zero")
	}
	if *concurrency <= 0 {
		log.Fatal("Concurrency must be greater than zero")
	}

	// Read BENCHMARK_KEY from env fallback if flag not provided
	effectiveBenchmarkKey := *benchmarkKey
	if effectiveBenchmarkKey == "" {
		effectiveBenchmarkKey = os.Getenv("BENCHMARK_KEY")
	}

	parsedBaseURL, parseErr := url.Parse(*endpoint)
	if parseErr != nil {
		log.Fatalf("Invalid endpoint URL %q: %v", *endpoint, parseErr)
	}

	pageBuckets, err := parsePageMix(*pageMix)
	if err != nil {
		log.Fatalf("Invalid page mix: %v", err)
	}

	endpointBuckets, err := parseEndpointMix(*endpointMix)
	if err != nil {
		log.Fatalf("Invalid endpoint mix: %v", err)
	}

	selectionSeed := *seed
	if selectionSeed == -1 {
		selectionSeed = time.Now().UnixNano()
	}
	selectedPages := selectPages(pageBuckets, *numRequests, selectionSeed)

	// Setup logging
	f, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	fmt.Printf("=========================================================\n")
	fmt.Printf("🏀 NBA_Go Enhanced Load Tester\n")
	fmt.Printf("=========================================================\n")
	fmt.Printf("Requests: %d | Concurrency: %d | Timeout: %v\n", *numRequests, *concurrency, *timeout)
	fmt.Printf("Base Target: %s\n", *endpoint)
	if *varyParams {
		fmt.Printf("Query Variance: ENABLED (Complexity: %s)\n", *complexity)
	} else {
		fmt.Printf("Query Variance: DISABLED (Legacy Mode)\n")
	}
	if *cacheBust {
		fmt.Printf("Cache Busting: ENABLED (100%% cold-cache origin benchmark)\n")
	}
	if effectiveBenchmarkKey != "" {
		fmt.Printf("Benchmark Auth: ENABLED (X-Benchmark-Key header attached)\n")
	}
	if *rotateIPs {
		fmt.Printf("Distributed IP Rotation: ENABLED (X-Real-IP / X-Forwarded-For)\n")
	}
	if len(endpointBuckets) > 0 {
		fmt.Printf("Endpoint Mix: %s\n", *endpointMix)
	}
	fmt.Printf("Logging to: %s\n\n", *logFile)

	// Channel to collect results
	results := make(chan Response, *numRequests)

	// Custom HTTP client with connection pooling and timeouts
	transport := &http.Transport{
		MaxIdleConns:        *concurrency * 2,
		MaxIdleConnsPerHost: *concurrency * 2,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   *timeout,
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, *concurrency)
	var activeReqSeq int64

	startTime := time.Now()

	for i := 0; i < *numRequests; i++ {
		wg.Add(1)
		go func(reqID int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			workerSeq := atomic.AddInt64(&activeReqSeq, 1)
			threadRNG := rand.New(rand.NewSource(selectionSeed + int64(reqID)*31 + workerSeq))

			targetPage := 0
			if len(selectedPages) > reqID {
				targetPage = selectedPages[reqID]
			}

			reqURL := GenerateRequestURL(RequestOptions{
				BaseURL:     *parsedBaseURL,
				Page:        targetPage,
				VaryParams:  *varyParams,
				Complexity:  *complexity,
				CacheBust:   *cacheBust,
				EndpointMix: endpointBuckets,
				RequestID:   reqID,
				WorkerID:    int(workerSeq),
				RNG:         threadRNG,
			})

			// If no param variation and pageMix was specified, preserve legacy URL logic exactly
			if !*varyParams && len(selectedPages) > 0 && !*cacheBust && len(endpointBuckets) == 0 {
				reqURL = urlForPage(*parsedBaseURL, targetPage)
			}

			maxAttempts := 1
			if *retryOnRateLimit {
				maxAttempts = 3
			}

			var finalResp Response
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				start := time.Now()
				req, err := http.NewRequestWithContext(context.Background(), "GET", reqURL, nil)
				if err != nil {
					finalResp = Response{
						Duration: time.Since(start),
						Error:    err,
						Page:     targetPage,
						URL:      reqURL,
						Attempt:  attempt,
					}
					log.Printf("Req %d: failed creating request: %v", reqID, err)
					break
				}

				if *apiKey != "" {
					req.Header.Set("x-api-key", *apiKey)
				}
				if effectiveBenchmarkKey != "" {
					req.Header.Set("X-Benchmark-Key", effectiveBenchmarkKey)
				}
				if *rotateIPs {
					simIP := fmt.Sprintf("198.51.100.%d", (reqID%250)+1)
					req.Header.Set("X-Real-IP", simIP)
					req.Header.Set("X-Forwarded-For", simIP)
				}

				httpResp, err := client.Do(req)
				duration := time.Since(start)

				if err != nil {
					finalResp = Response{
						Duration: duration,
						Error:    err,
						Page:     targetPage,
						URL:      reqURL,
						Attempt:  attempt,
					}
					log.Printf("Req %d (page %d) failed: %v", reqID, targetPage, err)
					break
				}

				bodyBytes, readErr := io.ReadAll(httpResp.Body)
				httpResp.Body.Close()

				cacheState := ExtractCacheStatus(httpResp.Header)

				finalResp = Response{
					StatusCode:  httpResp.StatusCode,
					BodyLength:  len(bodyBytes),
					Duration:    duration,
					Error:       readErr,
					Page:        targetPage,
					CacheStatus: cacheState,
					URL:         reqURL,
					Attempt:     attempt,
				}

				if httpResp.StatusCode == http.StatusTooManyRequests && *retryOnRateLimit && attempt < maxAttempts {
					backoff := time.Duration(300*attempt)*time.Millisecond + time.Duration(threadRNG.Intn(200))*time.Millisecond
					time.Sleep(backoff)
					continue
				}

				log.Printf("Req %d: Status=%d Cache=%s Duration=%v URL=%s", reqID, httpResp.StatusCode, cacheState, duration, reqURL)
				break
			}

			results <- finalResp
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregation variables
	var (
		totalResponses int
		successCount   int
		errorCount     int
		statusCounts   = make(map[int]int)

		allDurations   []time.Duration
		hitDurations   []time.Duration
		missDurations  []time.Duration
		otherDurations []time.Duration

		pageResults = make(map[int]*pageResult)
	)

	for res := range results {
		totalResponses++
		statusCounts[res.StatusCode]++

		if res.Error != nil || res.StatusCode < 200 || res.StatusCode >= 400 {
			errorCount++
		} else {
			successCount++
		}

		allDurations = append(allDurations, res.Duration)

		switch res.CacheStatus {
		case CacheHit:
			hitDurations = append(hitDurations, res.Duration)
		case CacheMiss:
			missDurations = append(missDurations, res.Duration)
		default:
			otherDurations = append(otherDurations, res.Duration)
		}

		if res.Page > 0 {
			stats, ok := pageResults[res.Page]
			if !ok {
				stats = &pageResult{}
				pageResults[res.Page] = stats
			}
			stats.requests++
			if res.Error != nil || res.StatusCode != http.StatusOK {
				stats.failures++
			} else {
				stats.successes++
				stats.totalDuration += res.Duration
			}
		}
	}

	totalTestDuration := time.Since(startTime)
	reqsPerSec := float64(totalResponses) / totalTestDuration.Seconds()

	overallStats := CalculatePercentiles(allDurations)
	hitStats := CalculatePercentiles(hitDurations)
	missStats := CalculatePercentiles(missDurations)

	// Summary output
	fmt.Printf("\n=========================================================\n")
	fmt.Printf("📊 LOAD TEST EXECUTION SUMMARY\n")
	fmt.Printf("=========================================================\n")
	fmt.Printf("Total Requests:       %d\n", totalResponses)
	fmt.Printf("Successful (2xx/3xx): %d (%.1f%%)\n", successCount, float64(successCount)*100.0/float64(totalResponses))
	fmt.Printf("Failed (4xx/5xx/Err): %d (%.1f%%)\n", errorCount, float64(errorCount)*100.0/float64(totalResponses))
	fmt.Printf("Total Duration:       %v\n", totalTestDuration)
	fmt.Printf("Throughput:           %.2f req/s\n\n", reqsPerSec)

	// Status code distribution
	fmt.Printf("--- HTTP Status Code Distribution ---\n")
	var sortedStatuses []int
	for code := range statusCounts {
		sortedStatuses = append(sortedStatuses, code)
	}
	sort.Ints(sortedStatuses)
	for _, code := range sortedStatuses {
		desc := http.StatusText(code)
		if desc == "" {
			desc = "Network / Connection Error"
		}
		fmt.Printf("  [%d %s]: %d\n", code, desc, statusCounts[code])
	}
	fmt.Println()

	// Latency percentiles breakdown
	fmt.Printf("--- Latency Percentile Distribution (All Responses) ---\n")
	fmt.Printf("  Min:    %v\n", overallStats.Min)
	fmt.Printf("  p50:    %v (Median)\n", overallStats.P50)
	fmt.Printf("  p75:    %v\n", overallStats.P75)
	fmt.Printf("  p90:    %v\n", overallStats.P90)
	fmt.Printf("  p95:    %v\n", overallStats.P95)
	fmt.Printf("  p99:    %v\n", overallStats.P99)
	fmt.Printf("  Max:    %v\n", overallStats.Max)
	fmt.Printf("  Avg:    %v\n\n", overallStats.Average)

	// Cache telemetry comparison
	fmt.Printf("--- Edge Cache vs Origin Latency Split ---\n")
	totalCacheTracked := len(hitDurations) + len(missDurations) + len(otherDurations)
	if totalCacheTracked > 0 {
		hitPct := float64(len(hitDurations)) * 100.0 / float64(totalCacheTracked)
		missPct := float64(len(missDurations)) * 100.0 / float64(totalCacheTracked)
		otherPct := float64(len(otherDurations)) * 100.0 / float64(totalCacheTracked)

		fmt.Printf("  ⚡ Cache HITS (Edge/CDN): %d (%.1f%%)\n", len(hitDurations), hitPct)
		if len(hitDurations) > 0 {
			fmt.Printf("     Avg: %v | p50: %v | p95: %v | p99: %v\n",
				hitStats.Average, hitStats.P50, hitStats.P95, hitStats.P99)
		}

		fmt.Printf("  🔥 Cache MISSES (Origin / DB): %d (%.1f%%)\n", len(missDurations), missPct)
		if len(missDurations) > 0 {
			fmt.Printf("     Avg: %v | p50: %v | p95: %v | p99: %v\n",
				missStats.Average, missStats.P50, missStats.P95, missStats.P99)
		}

		if len(otherDurations) > 0 {
			fmt.Printf("  ❓ Unclassified / Direct: %d (%.1f%%)\n", len(otherDurations), otherPct)
		}
	}
	fmt.Println()

	// Per-page results (if pageMix used)
	if len(pageResults) > 0 {
		pages := make([]int, 0, len(pageResults))
		for page := range pageResults {
			pages = append(pages, page)
		}
		sort.Ints(pages)

		fmt.Printf("--- Per-page Breakdown ---\n")
		for _, page := range pages {
			stats := pageResults[page]
			pageAverage := time.Duration(0)
			if stats.successes > 0 {
				pageAverage = stats.totalDuration / time.Duration(stats.successes)
			}
			fmt.Printf("  Page %2d: Requests=%4d, Success=%4d, Failed=%d, Avg Latency=%v\n",
				page, stats.requests, stats.successes, stats.failures, pageAverage)
		}
		fmt.Println()
	}
}

type pageResult struct {
	requests      int
	successes     int
	failures      int
	totalDuration time.Duration
}
