// Package request decodes and validates the body of an IP check.
package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/iso"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/netclass"
)

const maxSessionIDRunes = 128

const (
	fieldIP        = "ip_address"
	fieldCountries = "allowed_countries"
	fieldSession   = "session_id"
	fieldTime      = "requested_at"
)

// ErrMalformed means the body is not valid JSON.
var ErrMalformed = errors.New("request body is not valid JSON")

// Warning tells the caller about a value that was accepted but not used.
type Warning struct {
	Code    string `json:"code"`
	Detail  string `json:"detail"`
	Pointer string `json:"pointer,omitempty"`
}

// InvalidParam is one problem found in a request body.
type InvalidParam struct {
	Pointer string `json:"pointer"`
	Reason  string `json:"reason"`
}

// InvalidError lists every problem found in a request body.
type InvalidError struct {
	Params []InvalidParam
}

func (e *InvalidError) Error() string {
	return fmt.Sprintf("request has %d invalid parameters", len(e.Params))
}

// IgnoredField describes an optional field whose value was ignored. Length is
// set for a string; JSONType is set for any other JSON value.
type IgnoredField struct {
	Name     string
	Length   int
	JSONType string
}

// Request is a valid IP check request.
type Request struct {
	Addr          netip.Addr
	Countries     []string
	SessionID     string
	RequestedAt   string
	Warnings      []Warning
	Ignored       []IgnoredField
	UnknownFields []string
}

// Parse decodes and validates body. It returns ErrMalformed when the body is
// not JSON and an *InvalidError listing every problem when it is JSON but not
// a valid request.
func Parse(body []byte) (Request, error) {
	if !json.Valid(body) {
		return Request{}, ErrMalformed
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return Request{}, fmt.Errorf("decode request body: %w", err)
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return Request{}, &InvalidError{Params: []InvalidParam{{Pointer: "", Reason: "The request body must be a JSON object."}}}
	}

	var req Request
	var params []InvalidParam
	var p []InvalidParam

	req.Addr, p = parseAddr(fields[fieldIP])
	params = append(params, p...)
	var dups []Warning
	req.Countries, dups, p = parseCountries(fields[fieldCountries])
	params = append(params, p...)
	if len(params) > 0 {
		return Request{}, &InvalidError{Params: params}
	}

	req.Warnings = dups
	req.parseSessionID(fields[fieldSession], fields)
	req.parseRequestedAt(fields[fieldTime], fields)
	req.UnknownFields = unknownFields(fields)
	return req, nil
}

func parseAddr(v any) (netip.Addr, []InvalidParam) {
	const pointer = "/" + fieldIP
	if v == nil {
		return netip.Addr{}, []InvalidParam{{pointer, "ip_address is required."}}
	}
	s, ok := v.(string)
	if !ok {
		return netip.Addr{}, []InvalidParam{{pointer, "ip_address must be a string."}}
	}
	addr, err := netclass.Parse(s)
	if err != nil {
		return netip.Addr{}, []InvalidParam{{pointer, "ip_address must be a valid IPv4 or IPv6 address."}}
	}
	return addr, nil
}

// parseCountries returns the upper-cased codes without repeats, a warning for
// each repeat, or the problems found.
func parseCountries(v any) ([]string, []Warning, []InvalidParam) {
	const pointer = "/" + fieldCountries
	if v == nil {
		return nil, nil, []InvalidParam{{pointer, "allowed_countries is required."}}
	}
	list, ok := v.([]any)
	if !ok {
		return nil, nil, []InvalidParam{{pointer, "allowed_countries must be an array of strings."}}
	}
	if len(list) == 0 {
		return nil, nil, []InvalidParam{{pointer, "allowed_countries must contain at least one country code."}}
	}

	var codes []string
	var warnings []Warning
	var params []InvalidParam
	for i, item := range list {
		entryPointer := fmt.Sprintf("%s/%d", pointer, i)
		entry, ok := item.(string)
		if !ok {
			params = append(params, InvalidParam{entryPointer, "Each entry in allowed_countries must be a string."})
			continue
		}
		code := upperASCII(entry)
		switch {
		case !iso.Valid(code):
			params = append(params, InvalidParam{entryPointer, fmt.Sprintf("'%s' is not an ISO 3166-1 alpha-2 country code.", entry)})
		case slices.Contains(codes, code):
			warnings = append(warnings, Warning{
				Code:    "duplicate_country",
				Detail:  fmt.Sprintf("'%s' appears more than once in allowed_countries.", code),
				Pointer: entryPointer,
			})
		default:
			codes = append(codes, code)
		}
	}
	if len(params) > 0 {
		return nil, nil, params
	}
	return codes, warnings, nil
}

// upperASCII upper-cases only a-z, so that "ıt" never becomes "IT".
func upperASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'a' <= c && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

// parseSessionID keeps a string of at most 128 runes. An empty string means
// absent; any other value is ignored with a warning.
func (r *Request) parseSessionID(v any, fields map[string]any) {
	if _, present := fields[fieldSession]; !present {
		return
	}
	if s, ok := v.(string); ok && utf8.RuneCountInString(s) <= maxSessionIDRunes {
		r.SessionID = s
		return
	}
	r.ignore(fieldSession, v, "session_id must be a string of at most 128 characters; the value was ignored.")
}

// parseRequestedAt keeps a valid RFC 3339 timestamp; anything else is ignored
// with a warning.
func (r *Request) parseRequestedAt(v any, fields map[string]any) {
	if _, present := fields[fieldTime]; !present {
		return
	}
	if s, ok := v.(string); ok {
		if _, err := time.Parse(time.RFC3339, s); err == nil {
			r.RequestedAt = s
			return
		}
	}
	r.ignore(fieldTime, v, "requested_at must be a valid RFC 3339 timestamp; the value was ignored.")
}

func (r *Request) ignore(name string, v any, detail string) {
	r.Warnings = append(r.Warnings, Warning{Code: "invalid_" + name, Detail: detail, Pointer: "/" + name})
	field := IgnoredField{Name: name}
	if s, ok := v.(string); ok {
		field.Length = utf8.RuneCountInString(s)
	} else {
		field.JSONType = jsonType(v)
	}
	r.Ignored = append(r.Ignored, field)
}

func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case []any:
		return "array"
	default:
		return "object"
	}
}

func unknownFields(fields map[string]any) []string {
	var names []string
	for name := range fields {
		switch name {
		case fieldIP, fieldCountries, fieldSession, fieldTime:
		default:
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
