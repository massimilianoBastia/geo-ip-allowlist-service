// Package iso holds the constant list of officially assigned ISO 3166-1
// alpha-2 country codes and a membership check.
package iso

// Codes returns every officially assigned ISO 3166-1 alpha-2 code.
func Codes() []string {
	return nil
}

// Valid reports whether code is exactly one of the codes in Codes.
func Valid(code string) bool {
	return false
}
