package httpapi

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/request"
)

func TestProblemTypes(t *testing.T) {
	tests := []struct {
		got    problemType
		name   string
		title  string
		status int
	}{
		{typeMalformed, "malformed-request", "Malformed request", 400},
		{typeInvalid, "invalid-request", "Invalid request", 422},
		{typeTooLarge, "request-too-large", "Request too large", 413},
		{typeUnsupportedMedia, "unsupported-media-type", "Unsupported media type", 415},
		{typeMethodNotAllowed, "method-not-allowed", "Method not allowed", 405},
		{typeDatabaseMissing, "database-unavailable", "Service unavailable", 503},
		{typeInternal, "internal-error", "Internal error", 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := problemType{name: tt.name, title: tt.title, status: tt.status}
			if tt.got != want {
				t.Errorf("problem type = %+v, want %+v", tt.got, want)
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/x", nil)
			problems{baseURL: baseURL}.write(rec, req, tt.got, "Some detail.", nil)
			wantProblem(t, rec, tt.status, tt.name, tt.title, "Some detail.")
			if _, ok := decodeMap(t, rec)["invalid_params"]; ok {
				t.Error("invalid_params present without params")
			}
		})
	}
}

func TestProblemWithInvalidParams(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	params := []request.InvalidParam{{Pointer: "/ip_address", Reason: "r1"}, {Pointer: "/allowed_countries/0", Reason: "r2"}}
	problems{baseURL: baseURL}.write(rec, req, typeInvalid, "d", params)
	want := []any{
		map[string]any{"pointer": "/ip_address", "reason": "r1"},
		map[string]any{"pointer": "/allowed_countries/0", "reason": "r2"},
	}
	if got := decodeMap(t, rec)["invalid_params"]; !reflect.DeepEqual(got, want) {
		t.Errorf("invalid_params = %v, want %v", got, want)
	}
}

func TestProblemOnHeadHasNoBody(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/x", nil)
	problems{baseURL: baseURL}.write(rec, req, typeDatabaseMissing, "d", nil)
	if rec.Code != 503 || rec.Body.Len() != 0 {
		t.Errorf("got %d with %d body bytes, want 503 with none", rec.Code, rec.Body.Len())
	}
}
