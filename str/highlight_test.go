// SPDX-License-Identifier: MIT OR Unlicense

package str

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHighlightStringSimple(t *testing.T) {
	loc := [][2]int{{0, 4}}

	got := HighlightString("this", loc, "[in]", "[out]")

	expected := "[in]this[out]"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringCheckInOut(t *testing.T) {
	loc := [][2]int{{0, 4}}

	got := HighlightString("this", loc, "__", "__")

	expected := "__this__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringCheck2(t *testing.T) {
	loc := [][2]int{{0, 4}}

	got := HighlightString("bing", loc, "__", "__")

	expected := "__bing__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringCheckTwoWords(t *testing.T) {
	loc := [][2]int{{0, 4}, {5, 9}}

	got := HighlightString("this this", loc, "__", "__")

	expected := "__this__ __this__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringCheckMixedWords(t *testing.T) {
	loc := [][2]int{{0, 4}, {5, 9}, {10, 19}}

	got := HighlightString("this this something", loc, "__", "__")

	expected := "__this__ __this__ __something__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringOverlapStart(t *testing.T) {
	loc := [][2]int{{0, 1}, {0, 4}}

	got := HighlightString("THIS", loc, "__", "__")

	expected := "__THIS__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringOverlapMiddle(t *testing.T) {
	loc := [][2]int{{0, 4}, {1, 2}}

	got := HighlightString("this", loc, "__", "__")

	expected := "__this__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringNoOverlapMiddleNextSame(t *testing.T) {
	loc := [][2]int{{0, 1}, {1, 2}}

	got := HighlightString("this", loc, "__", "__")

	expected := "__t____h__is"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringOverlapMiddleLonger(t *testing.T) {
	loc := [][2]int{{0, 2}, {1, 4}}

	got := HighlightString("this", loc, "__", "__")

	expected := "__this__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestBugOne(t *testing.T) {
	loc := [][2]int{{10, 18}}

	got := HighlightString("this is unexpected", loc, "__", "__")

	expected := "this is un__expected__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func fromRegexpLoc(loc [][]int) [][2]int {
	l := make([][2]int, len(loc))
	for i, match := range loc {
		l[i] = [2]int{match[0], match[1]}
	}
	return l
}

func TestIntegrationRegex(t *testing.T) {
	r := regexp.MustCompile(`1`)
	haystack := "111"

	l := fromRegexpLoc(r.FindAllIndex([]byte(haystack), -1))
	got := HighlightString(haystack, l, "__", "__")

	if got != "__1____1____1__" {
		t.Error("Expected", "__1____1____1__", "got", got)
	}
}

func TestIntegrationIndexAll(t *testing.T) {
	haystack := "111"

	l := IndexAll(haystack, "1", -1)
	got := HighlightString(haystack, l, "__", "__")

	assert.Equal(t, "__1____1____1__", got)
}

func TestIntegrationIndexAllIgnoreCaseUnicode(t *testing.T) {
	haystack := "111"

	l := IndexAllIgnoreCase(haystack, "1", -1)
	got := HighlightString(haystack, l, "__", "__")

	assert.Equal(t, "__1____1____1__", got)
}
