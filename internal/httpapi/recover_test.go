package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func panicking(before func(*http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if before != nil {
			before(r)
		}
		panic("boom secret")
	})
}

func serveRecovered(h *harness, inner http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler := recoverPanics(h.logger, problems{baseURL: baseURL}, inner)
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/ip-checks", nil))
	return rec
}

func TestPanicBecomes500(t *testing.T) {
	h := newHarness(gbDB)
	rec := serveRecovered(h, panicking(nil))

	wantProblem(t, rec, 500, "internal-error", "Internal error", "An unexpected error occurred.")
	for _, leak := range []string{"boom", "secret", "goroutine"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("body %s contains %q", rec.Body, leak)
		}
	}
	recs := h.records(t)
	if len(recs) != 1 || recs[0]["level"] != "ERROR" {
		t.Fatalf("log records = %v, want one ERROR", recs)
	}
	if stack, _ := recs[0]["stack"].(string); stack == "" {
		t.Error("ERROR record has no stack")
	}
	if _, ok := recs[0]["session_id"]; ok {
		t.Error("record has a session_id before it was known")
	}
	if strings.Contains(h.logs.String(), "secret") {
		t.Error("the panic value was logged")
	}
}

func TestPanicLogsKnownSession(t *testing.T) {
	h := newHarness(gbDB)
	serveRecovered(h, panicking(func(r *http.Request) { setSessionID(r.Context(), "abc") }))

	recs := h.records(t)
	if len(recs) != 1 || recs[0]["session_id"] != "abc" {
		t.Errorf("log records = %v, want one with session_id=abc", recs)
	}
}

func TestPanicAfterHeadersAbortsTheConnection(t *testing.T) {
	h := newHarness(gbDB)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("boom secret")
	})

	var got any
	func() {
		defer func() { got = recover() }()
		serveRecovered(h, inner)
	}()
	if got != http.ErrAbortHandler {
		t.Errorf("recovered %v, want http.ErrAbortHandler", got)
	}
	if recs := h.records(t); len(recs) != 1 || recs[0]["level"] != "ERROR" {
		t.Errorf("log records = %v, want one ERROR", recs)
	}
}

func TestNextRequestAfterPanicSucceeds(t *testing.T) {
	h := newHarness(gbDB)
	calls := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			panic("boom")
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := recoverPanics(h.logger, problems{baseURL: baseURL}, inner)
	for i, want := range []int{500, 200} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != want {
			t.Errorf("request %d status = %d, want %d", i+1, rec.Code, want)
		}
	}
}

func TestNewRecoversPanicsFromTheDatabase(t *testing.T) {
	h := newHarness(panicDB{})
	rec := h.post(validBody)
	wantProblem(t, rec, 500, "internal-error", "Internal error", "An unexpected error occurred.")
}

func TestSessionOfOnePanicDoesNotLeakIntoTheNext(t *testing.T) {
	h := newHarness(gbDB)
	handler := recoverPanics(h.logger, problems{baseURL: baseURL}, panicking(func(r *http.Request) {
		if r.Header.Get("X-Session") != "" {
			setSessionID(r.Context(), r.Header.Get("X-Session"))
		}
	}))
	first := httptest.NewRequest(http.MethodPost, "/", nil)
	first.Header.Set("X-Session", "abc")
	handler.ServeHTTP(httptest.NewRecorder(), first)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))

	recs := h.records(t)
	if len(recs) != 2 {
		t.Fatalf("log records = %d, want 2", len(recs))
	}
	if _, ok := recs[1]["session_id"]; ok {
		t.Errorf("second record %v carries the session of the first", recs[1])
	}
}
