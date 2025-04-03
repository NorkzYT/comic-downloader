// Package grabber contains implementations and shared types for downloading comics.
package grabber

import (
	"regexp"
	"strconv"

	"github.com/spf13/pflag"
)

// getUuid extracts the first UUID found in the provided string.
// This helper is used by multiple grabber implementations to extract a unique identifier from a URL.
func getUuid(s string) string {
	// Note: the regex uses a non-capturing group for the hyphen parts.
	re := regexp.MustCompile(`([\w\d]{8}(?:-[\w\d]{4}){3}-[\w\d]{12})`)
	return re.FindString(s)
}

// maxUint8Flag returns the parsed flag value as uint8, capped at max.
func maxUint8Flag(flag *pflag.Flag, max uint8) uint8 {
	v, _ := strconv.ParseUint(flag.Value.String(), 10, 8)
	if v > uint64(max) {
		return max
	}
	return uint8(v)
}
