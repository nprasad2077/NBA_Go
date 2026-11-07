# Build context is the repo root so the shared go.mod/go.sum are available:
#   docker build -f cmd/cfexporter/Dockerfile .
FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/cfexporter ./cmd/cfexporter
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /cfexporter ./cmd/cfexporter

# distroless/static ships CA certificates for the HTTPS call to Cloudflare.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /cfexporter /cfexporter
EXPOSE 9200
ENTRYPOINT ["/cfexporter"]
