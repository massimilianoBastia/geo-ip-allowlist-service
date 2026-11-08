package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuditErrorPrecedenceWithoutDatabase(t *testing.T) {
	for _, tc := range []struct {
		name, method, media, body string
		status                    int
	}{
		{"method before media and size", "PUT", "text/plain", strings.Repeat("x", 16385), 405},
		{"media before size", "POST", "text/plain", strings.Repeat("x", 16385), 415},
		{"size before syntax", "POST", "application/json", strings.Repeat("x", 16385), 413},
		{"syntax before database", "POST", "application/json", "{", 400},
		{"validation before database", "POST", "application/json", "{}", 422},
		{"database before private address", "POST", "application/json", `{"ip_address":"10.0.0.1","allowed_countries":["GB"]}`, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newHarness(nil).do(tc.method, "/v1/ip-checks", tc.media, tc.body)
			if r.Code != tc.status || r.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("got %d %s: %s", r.Code, r.Header().Get("Content-Type"), r.Body.String())
			}
		})
	}
}

func TestAuditUnknownLengthBodyCap(t *testing.T) {
	for _, size := range []int{16384, 16385} {
		r := httptest.NewRequest(http.MethodPost, "/v1/ip-checks", strings.NewReader(validBody+strings.Repeat(" ", size-len(validBody))))
		r.ContentLength = -1 // The cap must also work for chunked requests.
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		newHarness(gbDB).handler.ServeHTTP(w, r)
		want := 200
		if size > 16384 {
			want = 413
		}
		if w.Code != want {
			t.Fatalf("size %d: got %d, want %d", size, w.Code, want)
		}
	}
}

func TestAuditDuplicateJSONKeysLastWins(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"ip_address":"bad","ip_address":"81.2.69.160","allowed_countries":["GB"]}`, 200},
		{`{"ip_address":"81.2.69.160","ip_address":"bad","allowed_countries":["GB"]}`, 422},
		{`{"ip_address":"81.2.69.160","allowed_countries":[],"allowed_countries":["GB"]}`, 200},
		{`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"allowed_countries":[]}`, 422},
	} {
		if r := newHarness(gbDB).post(tc.body); r.Code != tc.status {
			t.Fatalf("body %s: got %d, want %d", tc.body, r.Code, tc.status)
		}
	}
}
