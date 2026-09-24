package render

import "strings"

// QuotePOSIX quotes value as a single literal POSIX shell word.
func QuotePOSIX(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// QuoteMake doubles each $ so shell text reaches a Make recipe's shell unchanged.
func QuoteMake(value string) string {
	return strings.ReplaceAll(value, "$", "$$")
}
