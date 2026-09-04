package main

import (
	"flag"
	"fmt"
	"io/ioutil"
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
	"time"
)

// Response struct to track which server handled the request
type Response struct {
	StatusCode int
	Body       string
	Duration   time.Duration
	Error      error
	Page       int
}

type pageBucket struct {
	page   int
	weight float64
}

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

func urlForPage(base url.URL, page int) string {
	query := base.Query()
	query.Set("page", strconv.Itoa(page))
	base.RawQuery = query.Encode()
	return base.String()
}

func main() {
	// Define command line flags
	numRequests := flag.Int("n", 100, "Number of requests to send")
	concurrency := flag.Int("c", 10, "Number of concurrent requests")
	endpoint := flag.String("url", "http://localhost:8080/api/player/advanced", "API endpoint to test")
	logFile := flag.String("log", "loadtest.log", "Log file path")
	apiKey := flag.String("key", "", "API key for x-api-key header")
	pageMix := flag.String("pageMix", "", "Weighted page mix, for example 1-3:60,4-10:30,11-20:10")
	seed := flag.Int64("seed", -1, "Random seed for page selection (-1 uses the current time)")
	flag.Parse()

	if *numRequests <= 0 {
		log.Fatal("Number of requests must be greater than zero")
	}
	if *concurrency <= 0 {
		log.Fatal("Concurrency must be greater than zero")
	}

	pageBuckets, err := parsePageMix(*pageMix)
	if err != nil {
		log.Fatalf("Invalid page mix: %v", err)
	}

	var baseURL url.URL
	if len(pageBuckets) > 0 {
		parsedURL, parseErr := url.Parse(*endpoint)
		if parseErr != nil {
			log.Fatalf("Invalid endpoint URL: %v", parseErr)
		}
		baseURL = *parsedURL
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

	fmt.Printf("Starting load test with %d requests, %d concurrent\n", *numRequests, *concurrency)
	fmt.Printf("Testing endpoint: %s\n", *endpoint)

	// Channel to collect results
	results := make(chan Response, *numRequests)

	// Use a WaitGroup to manage concurrency
	var wg sync.WaitGroup

	// Semaphore to limit concurrency
	sem := make(chan bool, *concurrency)

	// Start the timer
	startTime := time.Now()

	// Launch goroutines for requests
	for i := 0; i < *numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Acquire semaphore
			sem <- true
			defer func() { <-sem }()

			start := time.Now()
			result := Response{
				Duration: 0,
				Error:    nil,
			}
			requestEndpoint := *endpoint
			if len(selectedPages) > 0 {
				result.Page = selectedPages[id]
				requestEndpoint = urlForPage(baseURL, result.Page)
			}

			req, err := http.NewRequest("GET", requestEndpoint, nil)
			if err != nil {
				result.Duration = time.Since(start)
				result.Error = err
				log.Printf("Request %d (page %d) failed to create request: %v", id, result.Page, err)
				results <- result
				return
			}

			if *apiKey != "" {
				req.Header.Add("x-api-key", *apiKey)
			}

			client := &http.Client{}
			resp, err := client.Do(req)
			duration := time.Since(start)

			result.Duration = duration
			result.Error = err

			if err != nil {
				log.Printf("Request %d (page %d) failed: %v", id, result.Page, err)
				results <- result
				return
			}

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Request %d (page %d) failed to read response body: %v", id, result.Page, err)
				result.Error = err
				results <- result
				return
			}

			result.StatusCode = resp.StatusCode
			result.Body = string(body)
			results <- result

			// Log request details
			log.Printf("Request %d: Page=%d, Status=%d, Time=%v", id, result.Page, resp.StatusCode, duration)
		}(i)
	}

	// Close the results channel when all requests are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	var successCount, errorCount int
	var totalDuration time.Duration
	pageResults := make(map[int]*pageResult)

	for result := range results {
		if result.Page > 0 {
			stats, ok := pageResults[result.Page]
			if !ok {
				stats = &pageResult{}
				pageResults[result.Page] = stats
			}
			stats.requests++
			if result.Error != nil || result.StatusCode != http.StatusOK {
				stats.failures++
			} else {
				stats.successes++
				stats.totalDuration += result.Duration
			}
		}

		if result.Error != nil {
			errorCount++
		} else if result.StatusCode == http.StatusOK {
			successCount++
			totalDuration += result.Duration
		} else {
			errorCount++
		}
	}

	// Calculate statistics
	totalTime := time.Since(startTime)
	var avgDuration time.Duration
	if successCount > 0 {
		avgDuration = totalDuration / time.Duration(successCount)
	}
	requestsPerSecond := float64(*numRequests) / totalTime.Seconds()

	// Print summary
	fmt.Printf("\nLoad Test Summary:\n")
	fmt.Printf("Total Requests: %d\n", *numRequests)
	fmt.Printf("Successful Requests: %d\n", successCount)
	fmt.Printf("Failed Requests: %d\n", errorCount)
	fmt.Printf("Total Time: %v\n", totalTime)
	fmt.Printf("Average Response Time: %v\n", avgDuration)
	fmt.Printf("Requests Per Second: %.2f\n", requestsPerSecond)

	if len(pageResults) > 0 {
		pages := make([]int, 0, len(pageResults))
		for page := range pageResults {
			pages = append(pages, page)
		}
		sort.Ints(pages)

		fmt.Printf("\nPer-page Results:\n")
		for _, page := range pages {
			stats := pageResults[page]
			pageAverage := time.Duration(0)
			if stats.successes > 0 {
				pageAverage = stats.totalDuration / time.Duration(stats.successes)
			}
			fmt.Printf("Page %d: Requests=%d, Successful=%d, Failed=%d, Average Response Time=%v\n",
				page, stats.requests, stats.successes, stats.failures, pageAverage)
		}
	}
}

type pageResult struct {
	requests      int
	successes     int
	failures      int
	totalDuration time.Duration
}
