package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Response struct to track which server handled the request
type Response struct {
	StatusCode int
	Body       string
	Duration   time.Duration
	Error      error
}

func main() {
	// Define command line flags
	numRequests := flag.Int("n", 100, "Number of requests to send")
	concurrency := flag.Int("c", 10, "Number of concurrent requests")
	endpoint := flag.String("url", "http://localhost:8080/api/player/advanced", "API endpoint to test")
	logFile := flag.String("log", "loadtest.log", "Log file path")
	apiKey := flag.String("key", "", "API key for x-api-key header")
	flag.Parse()

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

			req, err := http.NewRequest("GET", *endpoint, nil)
			if err != nil {
				result.Duration = time.Since(start)
				result.Error = err
				log.Printf("Request %d failed to create request: %v", id, err)
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
				log.Printf("Request %d failed: %v", id, err)
				results <- result
				return
			}

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Failed to read response body: %v", err)
				result.Error = err
				results <- result
				return
			}

			result.StatusCode = resp.StatusCode
			result.Body = string(body)
			results <- result

			// Log request details
			log.Printf("Request %d: Status=%d, Time=%v", id, resp.StatusCode, duration)
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

	for result := range results {
		if result.Error != nil {
			errorCount++
		} else if result.StatusCode == 200 {
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
}
