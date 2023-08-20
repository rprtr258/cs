// SPDX-License-Identifier: MIT OR Unlicense

package str

import (
	"strings"
	"unicode"
)

// RemoveStringDuplicates is a simple helper method that removes duplicates from
// any given str slice and then returns a nice duplicate free str slice
func RemoveStringDuplicates(elements []string) []string {
	encountered := map[string]struct{}{}
	var result []string
	for v := range elements {
		if _, ok := encountered[elements[v]]; !ok {
			encountered[elements[v]] = struct{}{}
			result = append(result, elements[v])
		}
	}
	return result
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
		s := ""
		for j, ch := range input {
			if i&(1<<j) == 0 {
				s += strings.ToUpper(string(ch))
			} else {
				s += strings.ToLower(string(ch))
			}
		}

		combinations = append(combinations, s)
	}
	return RemoveStringDuplicates(combinations)
}

// PermuteCaseFolding given a str returns a slice containing all possible case permutations
// with characters being folded such that S will return S s ſ
func PermuteCaseFolding(input string) []string {
	combinations := PermuteCase(input)

	var combos []string
	for _, combo := range combinations {
		for index, runeValue := range combo {
			for _, p := range AllSimpleFold(runeValue) {
				combos = append(combos, combo[:index]+string(p)+combo[index+len(string(runeValue)):])
			}
		}
	}
	return RemoveStringDuplicates(combos)
}

// AllSimpleFold given an input rune return a rune slice containing
// all of the possible simple fold
func AllSimpleFold(origin rune) []rune {
	c := origin
	res := []rune{origin}
	// This works for getting all folded representations
	// but feels totally wrong due to the bailout break.
	// That said its simpler than a while with checks
	// Investigate https://github.com/golang/go/blob/master/src/regexp/syntax/prog.go#L215 as a possible way to implement
	for i := 0; i < 255; i++ {
		c = unicode.SimpleFold(c)
		if c == origin {
			break
		}
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
	return (b < (0b1 << 7)) || ((0b11 << 6) < b)
}
