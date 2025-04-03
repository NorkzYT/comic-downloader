package ranges

import (
	"strconv"
	"strings"
)

// Range represents a range of chapter numbers using float64 to support decimal chapters.
type Range struct {
	Begin float64
	End   float64
}

// Parse parses a string representation of chapter ranges and returns a slice of Range.
// The expected formats are:
//   - A single chapter: "107.5"
//   - A range of chapters: "99-107.5"
//   - Multiple ranges separated by commas, e.g.: "1-10, 12, 15.5-20"
//
// Supports decimal chapter numbers.
func Parse(rnge string) (rngs []Range, err error) {
	// Split the input by commas to allow multiple ranges or individual chapters.
	parts := strings.Split(rnge, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue // Skip empty parts.
		}
		// If the part contains a dash, it denotes a range.
		if strings.Contains(part, "-") {
			// Split the part into beginning and ending chapter numbers.
			bounds := strings.Split(part, "-")
			if len(bounds) != 2 {
				// If we don't get exactly two parts, skip this part.
				continue
			}
			// Parse the beginning of the range.
			begin, err := strconv.ParseFloat(strings.TrimSpace(bounds[0]), 64)
			if err != nil {
				return nil, err
			}
			// Parse the end of the range.
			end, err := strconv.ParseFloat(strings.TrimSpace(bounds[1]), 64)
			if err != nil {
				return nil, err
			}
			// Ensure the range is valid (end should not be less than begin).
			if end < begin {
				end = begin
			}
			rngs = append(rngs, Range{
				Begin: begin,
				End:   end,
			})
		} else {
			// Otherwise, treat the part as a single chapter.
			val, err := strconv.ParseFloat(part, 64)
			if err != nil {
				return nil, err
			}
			rngs = append(rngs, Range{
				Begin: val,
				End:   val,
			})
		}
	}
	return rngs, nil
}
