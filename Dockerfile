# STEP 1 - build with CGO enabled
FROM golang:1.22-bullseye AS builder

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm64 

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o /nba_go .

# STEP 2 - final image
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Create data directory
RUN mkdir -p /app/data

COPY --from=builder /nba_go /nba_go

EXPOSE 5000

CMD ["/nba_go"]