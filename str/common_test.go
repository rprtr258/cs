package str

import (
	"fmt"
	"testing"

	"github.com/rprtr258/fun/iter"
	"github.com/stretchr/testify/assert"
)

func TestRemoveStringDuplicates(t *testing.T) {
	r := iter.FromMany("test", "test")
	assert.Equal(t, 1, iter.Count(RemoveStringDuplicates(r)))
}

func TestPermuteCase(t *testing.T) {
	assert.Equal(t, 4, iter.Count(PermuteCase("fo")))
}

func TestPermuteCaseUnicode(t *testing.T) {
	assert.Equal(t, 4, iter.Count(PermuteCase("ȺȾ")))
}

func TestPermuteCaseUnicodeNoFolding(t *testing.T) {
	assert.Equal(t, 2, iter.Count(PermuteCase("ſ")))
}

func TestAllSimpleFoldAsciiNumber(t *testing.T) {
	assert.Equal(t, 1, iter.Count(AllSimpleFold('1')))
}

func TestAllSimpleFoldAsciiLetter(t *testing.T) {
	folded := AllSimpleFold('z')
	assert.Equal(t, 2, iter.Count(folded))
}

func TestAllSimpleFoldMultipleReturn(t *testing.T) {
	folded := AllSimpleFold('ſ')
	assert.Equal(t, 3, iter.Count(folded))
}

func TestAllSimpleFoldNotFullFold(t *testing.T) {
	// ß (assuming I copied the lowercase one)
	// can with full fold rules turn into SS
	// https://www.w3.org/TR/charmod-norm/#definitionCaseFolding
	// however in this case its a simple fold
	// so we would not expect that
	folded := AllSimpleFold('ß')
	assert.Equal(t, 2, iter.Count(folded))
}

func TestPermuteCaseFoldingUnicodeNoFolding(t *testing.T) {
	assert.Equal(t, 3, iter.Count(PermuteCaseFolding("ſ")))
}

func TestPermuteCaseFolding(t *testing.T) {
	assert.Equal(t, 6, iter.Count(PermuteCaseFolding("nſ")))
}

func TestPermuteCaseFoldingNumbers(t *testing.T) {
	assert.Equal(t, 2, iter.Count(PermuteCaseFolding("07123E1")))
}

func TestPermuteCaseFoldingComparison(t *testing.T) {
	r1 := iter.Count(PermuteCase("groß"))
	r2 := iter.Count(PermuteCaseFolding("groß"))
	assert.NotEqual(t, r1, r2)
}

func TestIsSpace(t *testing.T) {
	for _, c := range []struct {
		b1, b2 byte
		want   bool
	}{
		// True cases
		{'\t', 'a', true},
		{'\n', 'a', true},
		{'\v', 'a', true},
		{'\f', 'a', true},
		{'\r', 'a', true},
		{' ', 'a', true},
		{'\xc2', '\x85', true}, // NEL
		{'\xc2', '\xa0', true}, // NBSP
		// False cases
		{'a', '\t', false},
		{byte(234), 'a', false},
		{byte(8), ' ', false},
		{'\xc2', byte(84), false},
		{'\xc2', byte(9), false},
	} {
		t.Run(fmt.Sprintf("%d-%d", c.b1, c.b2), func(t *testing.T) {
			assert.Equal(t, c.want, IsSpace(c.b1, c.b2))
		})
	}
}

func TestStartOfRune(t *testing.T) {
	for _, c := range []struct {
		bs   string
		idx  int
		want bool
	}{
		{"yo", 1, true},
		{"τoρνoς", 0, true},
		{"τoρνoς", 1, false},
		{"τoρνoς", 2, true},
		{"🍺", 0, true},
		{"🍺", 1, false},
		{"🍺", 2, false},
		{"🍺", 3, false},
	} {
		t.Run(fmt.Sprintf("%s-%d", string(c.bs), c.idx), func(t *testing.T) {
			assert.Equal(t, c.want, StartOfRune(c.bs[c.idx]))
		})
	}
}
