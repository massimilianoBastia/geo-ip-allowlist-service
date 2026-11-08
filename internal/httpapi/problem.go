package httpapi

import (
	"net/http"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/request"
)

// problemType is one RFC 9457 problem type of the service.
type problemType struct {
	name   string
	title  string
	status int
}

var (
	typeMalformed        = problemType{"malformed-request", "Malformed request", http.StatusBadRequest}
	typeInvalid          = problemType{"invalid-request", "Invalid request", http.StatusUnprocessableEntity}
	typeTooLarge         = problemType{"request-too-large", "Request too large", http.StatusRequestEntityTooLarge}
	typeUnsupportedMedia = problemType{"unsupported-media-type", "Unsupported media type", http.StatusUnsupportedMediaType}
	typeMethodNotAllowed = problemType{"method-not-allowed", "Method not allowed", http.StatusMethodNotAllowed}
	typeDatabaseMissing  = problemType{"database-unavailable", "Service unavailable", http.StatusServiceUnavailable}
	typeInternal         = problemType{"internal-error", "Internal error", http.StatusInternalServerError}
)

var allProblemTypes = []problemType{
	typeMalformed, typeInvalid, typeTooLarge, typeUnsupportedMedia,
	typeMethodNotAllowed, typeDatabaseMissing, typeInternal,
}

// problemDocument is the JSON body of a problem response.
type problemDocument struct {
	Type          string                 `json:"type"`
	Title         string                 `json:"title"`
	Status        int                    `json:"status"`
	Detail        string                 `json:"detail"`
	InvalidParams []request.InvalidParam `json:"invalid_params,omitempty"`
}

// problems writes RFC 9457 documents.
type problems struct {
	baseURL string
	metrics *metrics
}

// write sends the problem document with the status of t.
func (p problems) write(w http.ResponseWriter, r *http.Request, t problemType, detail string, params []request.InvalidParam) {
	p.metrics.countProblem(t)
	writeJSON(w, r, t.status, "application/problem+json", problemDocument{
		Type:          p.baseURL + t.name,
		Title:         t.title,
		Status:        t.status,
		Detail:        detail,
		InvalidParams: params,
	})
}
