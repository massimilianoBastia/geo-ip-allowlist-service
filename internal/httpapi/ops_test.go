package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestLivez(t *testing.T) {
	for name, h := range map[string]*harness{"ready": newHarness(gbDB), "not ready": newHarness(nil)} {
		t.Run(name, func(t *testing.T) {
			rec := h.do(http.MethodGet, "/livez", "", "")
			if rec.Code != 200 {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got, want := decodeMap(t, rec), map[string]any{"status": "ok"}; !reflect.DeepEqual(got, want) {
				t.Errorf("body = %v, want %v", got, want)
			}
		})
	}
}

func TestReadyz(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		rec := newHarness(gbDB).do(http.MethodGet, "/readyz", "", "")
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		want := map[string]any{"status": "ready", "database_type": "GeoLite2-Country", "database_built_at": "2026-01-02T03:04:05Z"}
		if got := decodeMap(t, rec); !reflect.DeepEqual(got, want) {
			t.Errorf("body = %v, want %v", got, want)
		}
	})
	t.Run("not ready", func(t *testing.T) {
		rec := newHarness(nil).do(http.MethodGet, "/readyz", "", "")
		wantProblem(t, rec, 503, "database-unavailable", "Service unavailable", "The geolocation database is not loaded yet; retry later.")
	})
}

func TestVersion(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		rec := newHarness(gbDB).do(http.MethodGet, "/version", "", "")
		want := map[string]any{"version": "1.2.3", "database_built_at": "2026-01-02T03:04:05Z"}
		if got := decodeMap(t, rec); rec.Code != 200 || !reflect.DeepEqual(got, want) {
			t.Errorf("got %d %v, want 200 %v", rec.Code, got, want)
		}
	})
	t.Run("not ready omits the build date", func(t *testing.T) {
		rec := newHarness(nil).do(http.MethodGet, "/version", "", "")
		want := map[string]any{"version": "1.2.3"}
		if got := decodeMap(t, rec); rec.Code != 200 || !reflect.DeepEqual(got, want) {
			t.Errorf("got %d %v, want 200 %v", rec.Code, got, want)
		}
	})
}

func TestOpsOnlyAllowGetAndHead(t *testing.T) {
	for _, path := range []string{"/livez", "/readyz", "/version"} {
		t.Run(path, func(t *testing.T) {
			rec := newHarness(gbDB).do(http.MethodPost, path, "", "")
			wantProblem(t, rec, 405, "method-not-allowed", "Method not allowed", "This resource only supports GET and HEAD.")
			if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
				t.Errorf("Allow = %q, want %q", got, "GET, HEAD")
			}
		})
	}
}

func TestOpsHeadHasNoBody(t *testing.T) {
	for _, path := range []string{"/livez", "/readyz", "/version"} {
		t.Run(path, func(t *testing.T) {
			rec := newHarness(gbDB).do(http.MethodHead, path, "", "")
			if rec.Code != 200 || rec.Body.Len() != 0 {
				t.Errorf("got %d with %d body bytes, want 200 with none", rec.Code, rec.Body.Len())
			}
		})
	}
}

func TestNoHealthz(t *testing.T) {
	if rec := newHarness(gbDB).do(http.MethodGet, "/healthz", "", ""); rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestProbesWriteDebugSummaryOnly(t *testing.T) {
	for _, path := range []string{"/livez", "/readyz", "/version", "/metrics"} {
		t.Run(path, func(t *testing.T) {
			h := newHarness(gbDB)
			h.do(http.MethodGet, path, "", "")
			sum := h.summary(t)
			if sum["level"] != "DEBUG" || sum["path"] != path || sum["status"] != float64(200) || sum["method"] != "GET" {
				t.Errorf("summary = %v, want DEBUG GET %s 200", sum, path)
			}
			if _, ok := sum["duration_ms"]; !ok {
				t.Error("summary has no duration_ms")
			}
		})
	}
	t.Run("405 and 503 are logged too", func(t *testing.T) {
		h := newHarness(nil)
		h.do(http.MethodGet, "/readyz", "", "")
		if sum := h.summary(t); sum["status"] != float64(503) || sum["level"] != "DEBUG" {
			t.Errorf("summary = %v, want DEBUG 503", sum)
		}
	})
	t.Run("nothing at INFO level", func(t *testing.T) {
		logs := &bytes.Buffer{}
		handler := New(fakeSource{db: gbDB}, Settings{MaxBodyBytes: 16384, ProblemBaseURL: baseURL}, slog.New(slog.NewJSONHandler(logs, nil)))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))
		if logs.Len() != 0 {
			t.Errorf("logs at INFO = %q, want none", logs)
		}
	})
}
