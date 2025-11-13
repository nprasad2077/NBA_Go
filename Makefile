# Tells 'make' these are command names, not files
.PHONY: up down

# Command to run: make up
up:
	docker-compose -f docker-compose.local.yml up --build -d

# Command to run: make down
down:
	docker-compose down