package app

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerEndpoints(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	info := BuildInfo{Version: "1.0.0", Commit: "abc1234", Date: "2026-01-01"}
	srv := NewServer(info, logger)
	handler := srv.Handler()

	// 1. Test /
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /, got %d", rec.Code)
	}

	// 2. Test /healthz
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected /healthz response: %d %s", rec.Code, rec.Body.String())
	}

	// 3. Test /readyz (avant SetReady(true) -> 503)
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for /readyz before ready, got %d", rec.Code)
	}

	// 4. Test /readyz (après SetReady(true) -> 200)
	srv.SetReady(true)
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ready" {
		t.Fatalf("expected 200 for /readyz after ready, got %d", rec.Code)
	}

	// 5. Test /version
	req = httptest.NewRequest(http.MethodGet, "/version", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /version, got %d", rec.Code)
	}
	var got BuildInfo
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil || got != info {
		t.Fatalf("unexpected /version body: %+v (err=%v)", got, err)
	}

	// 6. Test NewServer avec logger nil
	srvDefault := NewServer(info, nil)
	if srvDefault.logger == nil {
		t.Fatal("expected default logger when nil is passed")
	}
}
