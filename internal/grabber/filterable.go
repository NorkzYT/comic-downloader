package grabber

import (
	"sort"

	"github.com/NorkzYT/comic-downloader/internal/ranges"
)

// Enumerable represents an object that can be enumerated
type Enumerable interface {
	GetNumber() float64
}

// Titleable represents an object that can be titled
type Titleable interface {
	GetTitle() string
}

// Filterable represents a filterable object
type Filterable interface {
	Enumerable
	Titleable
}

// Filterables represents a slice of Filterable
type Filterables []Filterable

// Filter allows filtering Filterables by the given condition.
func (f Filterables) Filter(cond func(Filterable) bool) Filterables {
	filtered := Filterables{}
	for _, chap := range f {
		if cond(chap) {
			filtered = append(filtered, chap)
		}
	}

	return filtered
}

// FilterRanges returns the specified ranges of Filterables sorted by their Number.
// It now directly compares float64 values.
func (f Filterables) FilterRanges(rngs []ranges.Range) Filterables {
	chaps := Filterables{}
	for _, r := range rngs {
		chaps = append(chaps, f.Filter(func(c Filterable) bool {
			return c.GetNumber() >= r.Begin && c.GetNumber() <= r.End
		})...)
	}

	return chaps
}

// SortByNumber sorts Filterables by Number.
func (f Filterables) SortByNumber() Filterables {
	sort.Slice(f, func(i, j int) bool {
		return f[i].GetNumber() < f[j].GetNumber()
	})

	return f
}
