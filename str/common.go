package str

import (
	"strings"
	"unicode"
)

// RemoveStringDuplicates is a simple helper method that removes duplicates from
// any given str slice and then returns a nice duplicate free str slice
func RemoveStringDuplicates(elements []string) []string {
	uniques := map[string]struct{}{}
	for v := range elements {
		uniques[elements[v]] = struct{}{}
	}

	res := make([]string, 0, len(uniques))
	for v := range uniques {
		res = append(res, v)
	}
	return res
}

// PermuteCase given a str returns a slice containing all possible case permutations
// of that str such that input of foo will return
// foo Foo fOo FOo foO FoO fOO FOO
// Note that very long inputs can produce an enormous amount of
// results in the returned slice OR result in an overflow and return nothing
func PermuteCase(input string) []string {
	max := 1 << len(input)

	var combinations []string
	for i := 0; i < max; i++ {
		var sb strings.Builder
		for j, ch := range input { // TODO: j is byte index, so might produce duplicates
			if i&(1<<j) == 0 {
				sb.WriteRune(unicode.ToUpper(ch))
			} else {
				sb.WriteRune(unicode.ToLower(ch))
			}
		}
		combinations = append(combinations, sb.String())
	}
	return RemoveStringDuplicates(combinations)
}

// PermuteCaseFolding given a str returns a slice containing all possible case permutations
// with characters being folded such that S will return S s ſ
func PermuteCaseFolding(input string) []string {
	var combos []string
	for _, combo := range PermuteCase(input) {
		for i, ch := range combo {
			for _, p := range AllSimpleFold(ch) {
				combos = append(combos, combo[:i]+string(p)+combo[i+len(string(ch)):])
			}
		}
	}
	return RemoveStringDuplicates(combos)
}

// AllSimpleFold given an input rune return a rune slice containing
// all of the possible simple fold
func AllSimpleFold(origin rune) []rune {
	res := []rune{origin}
	// This works for getting all folded representations
	// but feels totally wrong due to the bailout break.
	// That said its simpler than a while with checks
	// Investigate https://github.com/golang/go/blob/master/src/regexp/syntax/prog.go#L215 as a possible way to implement
	for c, i := unicode.SimpleFold(origin), 0; i < 255 && c != origin; c, i = unicode.SimpleFold(c), i+1 {
		res = append(res, c)
	}
	return res
}

// IsSpace checks bytes MUST which be UTF-8 encoded for a space
// List of spaces detected (same as unicode.IsSpace):
// '\t', '\n', '\v', '\f', '\r', ' ', U+0085 (NEL), U+00A0 (NBSP).
// N.B only two bytes are required for these cases.  If we decided
// to support spaces like '，' then we'll need more bytes.
func IsSpace(firstByte, nextByte byte) bool {
	const (
		SPACE = 32
		NEL   = 133
		NBSP  = 160
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
