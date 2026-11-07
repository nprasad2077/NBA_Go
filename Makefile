# Tells 'make' these are command names, not files
.PHONY: up down loadtest loadtest-quick loadtest-build loadtest-build-linux

# Command to run: make up
up:
	docker-compose -f docker-compose.local.yml up --build -d

# Command to run: make down
down:
	docker-compose down

# ==============================================================================
# Load Testing
# ==============================================================================
API_HOST ?= https://nba.space-city.dev
SCENARIO ?= all

# Run full load test suite (or specific scenario with SCENARIO=cold, warm, mix, variance)
loadtest:
	API_HOST=$(API_HOST) ./test/run_suite.sh $(SCENARIO)

# Quick smoke test with reduced request counts
loadtest-quick:
	API_HOST=$(API_HOST) ./test/run_suite.sh $(SCENARIO) --quick

# Build the local loadtest binary
loadtest-build:
	go build -o test/bin/loadtest ./test/loadtest.go

# Cross-compile standalone Linux binary for running from any cloud VM (EC2, Droplet, etc.)
loadtest-build-linux:
	GOOS=linux GOARCH=amd64 go build -o test/bin/loadtest-linux ./test/loadtest.go
