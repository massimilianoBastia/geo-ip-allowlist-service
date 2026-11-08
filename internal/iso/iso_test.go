package iso_test

import (
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/iso"
)

func TestCodes(t *testing.T) {
	codes := iso.Codes()
	// Stop here on a wrong count so the checks below can never pass on an
	// empty list.
	if len(codes) != 249 {
		t.Fatalf("len(Codes()) = %d, want 249", len(codes))
	}

	seen := make(map[string]bool, len(codes))
	for _, c := range codes {
		if !isTwoUpperLetters(c) {
			t.Errorf("code %q is not two upper-case ASCII letters", c)
		}
		if seen[c] {
			t.Errorf("code %q appears more than once", c)
		}
		seen[c] = true
	}

	tests := []struct {
		code    string
		present bool
	}{
		{"GB", true},
		{"IT", true},
		{"UK", false},
		{"XK", false},
	}
	for _, tt := range tests {
		if got := seen[tt.code]; got != tt.present {
			t.Errorf("%q in Codes() = %v, want %v", tt.code, got, tt.present)
		}
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{"assigned code GB", "GB", true},
		{"assigned code IT", "IT", true},
		{"lower case", "gb", false},
		{"exceptionally reserved UK", "UK", false},
		{"user-assigned XK", "XK", false},
		{"three letters", "USA", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := iso.Valid(tt.code); got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func isTwoUpperLetters(s string) bool {
	return len(s) == 2 && 'A' <= s[0] && s[0] <= 'Z' && 'A' <= s[1] && s[1] <= 'Z'
}
