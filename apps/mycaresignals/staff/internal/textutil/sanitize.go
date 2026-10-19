package textutil

import "strings"

// SanitizeForPostgres makes text safe to store in a Postgres TEXT column.
func SanitizeForPostgres(s string) string {
	// 1) Fix invalid UTF-8 sequences (e.g. stray 0xB5 etc.)
	s = strings.ToValidUTF8(s, " ")

	// 2) Strip NUL bytes (0x00) – Postgres TEXT does not allow these
	s = strings.ReplaceAll(s, "\x00", "")

	return s
}
