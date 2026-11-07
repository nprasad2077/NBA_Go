package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeCF struct {
	fail   bool
	calls  []map[string]string
	server *httptest.Server
}

func newFakeCF(t *testing.T) *fakeCF {
	f := &fakeCF{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body struct {
			Variables map[string]string `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		f.calls = append(f.calls, body.Variables)
		if f.fail {
			w.Write([]byte(`{"data":null,"errors":[{"message":"Rate limiter budget depleted"}]}`))
			return
		}
		w.Write([]byte(`{"data":{"viewer":{"zones":[{
			"api":[{"count":10,"sum":{"edgeResponseBytes":1000},"dimensions":{"cacheStatus":"hit"}},
			       {"count":2,"sum":{"edgeResponseBytes":300},"dimensions":{"cacheStatus":"miss"}}],
			"other":[{"count":5,"sum":{"edgeResponseBytes":50},"dimensions":{"cacheStatus":"dynamic"}}]
		}]}},"errors":null}`))
	}))
	t.Cleanup(f.server.Close)
	return f
}

func testExporter(url string) *exporter {
	return newExporter(config{
		token: "test-token", zoneID: "z", host: "nba.example", url: url,
		interval: 5 * time.Minute, delay: 2 * time.Minute,
	}, prometheus.NewRegistry())
}

func TestPollAddsCountsAndAdvancesWindow(t *testing.T) {
	f := newFakeCF(t)
	e := testExporter(f.server.URL)
	now := time.Date(2026, 11, 4, 12, 0, 30, 0, time.UTC)

	if err := e.poll(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(e.requests.WithLabelValues("nba.example", "api", "hit")); got != 10 {
		t.Errorf("api hit = %v, want 10", got)
	}
	if got := testutil.ToFloat64(e.requests.WithLabelValues("nba.example", "other", "dynamic")); got != 5 {
		t.Errorf("other dynamic = %v, want 5", got)
	}
	if got := testutil.ToFloat64(e.bytes.WithLabelValues("nba.example", "api", "miss")); got != 300 {
		t.Errorf("api miss bytes = %v, want 300", got)
	}
	// First window: [11:53, 11:58) — end is now-2m truncated to the minute.
	if f.calls[0]["since"] != "2026-11-04T11:53:00Z" || f.calls[0]["until"] != "2026-11-04T11:58:00Z" {
		t.Errorf("first window = %v", f.calls[0])
	}

	// Second poll 5 minutes later continues exactly where the first ended.
	if err := e.poll(context.Background(), now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if f.calls[1]["since"] != "2026-11-04T11:58:00Z" || f.calls[1]["until"] != "2026-11-04T12:03:00Z" {
		t.Errorf("second window = %v", f.calls[1])
	}
	if got := testutil.ToFloat64(e.requests.WithLabelValues("nba.example", "api", "hit")); got != 20 {
		t.Errorf("api hit after two polls = %v, want 20", got)
	}
}

func TestFailedPollDoesNotAdvanceWindow(t *testing.T) {
	f := newFakeCF(t)
	e := testExporter(f.server.URL)
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	e.poll(context.Background(), now)
	f.fail = true
	if err := e.poll(context.Background(), now.Add(5*time.Minute)); err == nil {
		t.Fatal("expected error from rate-limited poll")
	}
	if got := testutil.ToFloat64(e.errors); got != 1 {
		t.Errorf("errors = %v, want 1", got)
	}
	f.fail = false
	e.poll(context.Background(), now.Add(10*time.Minute))
	// Catch-up window spans both the failed and the current interval.
	last := f.calls[len(f.calls)-1]
	if last["since"] != "2026-11-04T11:58:00Z" || last["until"] != "2026-11-04T12:08:00Z" {
		t.Errorf("catch-up window = %v", last)
	}
}

func TestWindowCappedAtMax(t *testing.T) {
	e := testExporter("")
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	e.lastEnd = now.Add(-48 * time.Hour)
	start, end, ok := e.window(now)
	if !ok || end.Sub(start) != maxWindow {
		t.Errorf("window = %v..%v ok=%v, want %v span", start, end, ok, maxWindow)
	}
}

func TestBadTokenIsError(t *testing.T) {
	f := newFakeCF(t)
	e := testExporter(f.server.URL)
	e.cfg.token = "wrong"
	if err := e.poll(context.Background(), time.Now()); err == nil {
		t.Fatal("expected error for unauthorized token")
	}
}
