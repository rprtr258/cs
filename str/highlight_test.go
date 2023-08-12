package str

import (
	"regexp"
	"testing"

	"github.com/rprtr258/fun"
	"github.com/rprtr258/fun/iter"
)

func TestHighlightStringSimple(t *testing.T) {
	loc := iter.FromMany[[2]int]([2]int{0, 4})

	got := HighlightString("this", loc, "[in]", "[out]")

	expected := "[in]this[out]"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringCheckInOut(t *testing.T) {
	loc := iter.FromMany[[2]int]([2]int{0, 4})

	got := HighlightString("this", loc, "__", "__")

	expected := "__this__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringCheck2(t *testing.T) {
	loc := iter.FromMany[[2]int]([2]int{0, 4})

	got := HighlightString("bing", loc, "__", "__")

	expected := "__bing__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringCheckTwoWords(t *testing.T) {
	loc := iter.FromMany[[2]int]([2]int{0, 4}, [2]int{5, 9})

	got := HighlightString("this this", loc, "__", "__")

	expected := "__this__ __this__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringCheckMixedWords(t *testing.T) {
	loc := iter.FromMany[[2]int](
		[2]int{0, 4},
		[2]int{5, 9},
		[2]int{10, 19},
	)

	got := HighlightString("this this something", loc, "__", "__")

	expected := "__this__ __this__ __something__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringOverlapStart(t *testing.T) {
	loc := iter.FromMany[[2]int](
		[2]int{0, 1},
		[2]int{0, 4},
	)

	got := HighlightString("THIS", loc, "__", "__")

	expected := "__THIS__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringOverlapMiddle(t *testing.T) {
	loc := iter.FromMany[[2]int](
		[2]int{0, 4},
		[2]int{1, 2},
	)

	got := HighlightString("this", loc, "__", "__")

	expected := "__this__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringNoOverlapMiddleNextSame(t *testing.T) {
	loc := iter.FromMany[[2]int](
		[2]int{0, 1},
		[2]int{1, 2},
	)

	got := HighlightString("this", loc, "__", "__")

	expected := "__t____h__is"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestHighlightStringOverlapMiddleLonger(t *testing.T) {
	loc := iter.FromMany[[2]int](
		[2]int{0, 2},
		[2]int{1, 4},
	)

	got := HighlightString("this", loc, "__", "__")

	expected := "__this__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestBugOne(t *testing.T) {
	loc := iter.FromMany[[2]int]([2]int{10, 18})

	got := HighlightString("this is unexpected", loc, "__", "__")

	expected := "this is un__expected__"
	if got != expected {
		t.Error("Expected", expected, "got", got)
	}
}

func TestIntegrationRegex(t *testing.T) {
	r := regexp.MustCompile(`1`)
	haystack := "111"

	loc := r.FindAllIndex([]byte(haystack), -1)

	locloc := iter.Map(iter.FromSlice(loc), func(v fun.Pair[int, []int]) [2]int {
		return [2]int{v.V[0], v.V[1]}
	})

	got := HighlightString(haystack, locloc, "__", "__")

	if got != "__1____1____1__" {
		t.Error("Expected", "__1____1____1__", "got", got)
	}
}

func TestIntegrationIndexAll(t *testing.T) {
	haystack := "111"

	loc := IndexAll(haystack, "1")
	got := HighlightString(haystack, loc, "__", "__")

	if got != "__1____1____1__" {
		t.Error("Expected", "__1____1____1__", "got", got)
	}
}

func TestIntegrationIndexAllIgnoreCaseUnicode(t *testing.T) {
	haystack := "111"

	loc := IndexAllIgnoreCase(haystack, "1")
	got := HighlightString(haystack, loc, "__", "__")

	if got != "__1____1____1__" {
		t.Error("Expected", "__1____1____1__", "got", got)
	}
}
