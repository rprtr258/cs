package str

import (
	"strings"
	"unicode"

	"github.com/rprtr258/fun/iter"
)

// RemoveStringDuplicates is a simple helper method that removes duplicates from
// any given str slice and then returns a nice duplicate free str slice
func RemoveStringDuplicates(elements iter.Seq[string]) iter.Seq[string] {
	return func(yield func(string) bool) bool {
		encountered := map[string]struct{}{}
		return elements(func(v string) bool {
			if _, ok := encountered[v]; !ok {
				encountered[v] = struct{}{}
				if !yield(v) {
					return false
				}
			}
			return true
		})
	}
}

// PermuteCase given a str returns a slice containing all possible case permutations
// of that str such that input of foo will return
// foo Foo fOo FOo foO FoO fOO FOO
// Note that very long inputs can produce an enormous amount of
// results in the returned slice OR result in an overflow and return nothing
func PermuteCase(input string) iter.Seq[string] {
	l := len(input)
	max := 1 << l

	combinations := func(yield func(string) bool) bool {
		for i := 0; i < max; i++ {
			s := ""
			for j, ch := range input {
				if (i & (1 << j)) == 0 {
					s += strings.ToUpper(string(ch))
				} else {
					s += strings.ToLower(string(ch))
				}
			}

			if !yield(s) {
				return false
			}
		}
		return true
	}

	return RemoveStringDuplicates(combinations)
}

// PermuteCaseFolding given a str returns a slice containing all possible case permutations
// with characters being folded such that S will return S s ſ
func PermuteCaseFolding(input string) iter.Seq[string] {
	combinations := PermuteCase(input)
	combos := iter.FlatMap(
		combinations,
		func(combo string) iter.Seq[string] {
			return func(yield func(string) bool) bool {
				for i, runeValue := range combo {
					AllSimpleFold(runeValue)(func(p rune) bool {
						return yield(combo[:i] + string(p) + combo[i+len(string(runeValue)):])
					})
				}
				return true
			}
		})

	return RemoveStringDuplicates(combos)
}

// AllSimpleFold given an input rune return a rune slice containing
// all of the possible simple fold
func AllSimpleFold(input rune) iter.Seq[rune] {
	origin := input

	return func(yield func(rune) bool) bool {
		if !yield(origin) {
			return false
		}
		// This works for getting all folded representations
		// but feels totally wrong due to the bailout break.
		// That said its simpler than a while with checks
		// Investigate https://github.com/golang/go/blob/master/src/regexp/syntax/prog.go#L215 as a possible way to implement
		input := input
		for {
			input = unicode.SimpleFold(input)
			if input == origin {
				return true
			}
			if !yield(input) {
				return false
			}
		}
	}
}

// IsSpace checks bytes MUST which be UTF-8 encoded for a space
// List of spaces detected (same as unicode.IsSpace):
// '\t', '\n', '\v', '\f', '\r', ' ', U+0085 (NEL), U+00A0 (NBSP).
// N.B only two bytes are required for these cases.  If we decided
// to support spaces like '，' then we'll need more bytes.
func IsSpace(firstByte, nextByte byte) bool {
	const (
		NEL   = 133
		NBSP  = 160
		SPACE = 32
	)
	return 9 <= firstByte && firstByte <= 13 || // \t, \n, \f, \r
		firstByte == SPACE ||
		firstByte == 194 && (nextByte == NEL || nextByte == NBSP)
}

// StartOfRune a byte and returns true if its the start of a multibyte
// character or a single byte character otherwise false
func StartOfRune(b byte) bool {
	return b < 0b10000000 || 0b11000000 < b
}
