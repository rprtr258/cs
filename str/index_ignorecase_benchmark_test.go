package str

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rprtr258/fun/iter"
	"github.com/stretchr/testify/assert"
)

var (
	_large101 = strings.Repeat(testUnicodeMatchEndCaseLarge, 101)
	_large11  = strings.Repeat(testUnicodeMatchEndCaseLarge, 11)
)

func BenchmarkFindAllIndexCaseInsensitive(b *testing.B) {
	r := regexp.MustCompile(`(?i)test`)
	haystack := []byte(testMatchEndCase)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 1, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseCaseInsensitive(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(testMatchEndCase, "test")

		assert.Equal(b, 1, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexLargeCaseInsensitive(b *testing.B) {
	r := regexp.MustCompile(`(?i)test`)
	haystack := []byte(testMatchEndCaseLarge)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 1, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseLargeCaseInsensitive(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(testMatchEndCaseLarge, "test")
		assert.Equal(b, 1, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexUnicodeCaseInsensitive(b *testing.B) {
	r := regexp.MustCompile(`(?i)test`)
	haystack := []byte(testUnicodeMatchEndCase)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 1, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseUnicodeCaseInsensitive(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(testUnicodeMatchEndCase, "test")
		assert.Equal(b, 1, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexUnicodeLargeCaseInsensitive(b *testing.B) {
	r := regexp.MustCompile(`(?i)test`)
	haystack := []byte(testUnicodeMatchEndCaseLarge)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 1, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseUnicodeLargeCaseInsensitive(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(testUnicodeMatchEndCaseLarge, "test")
		assert.Equal(b, 1, iter.Count(matches))
	}
}

// This benchmark simulates a bad case of there being many
// partial matches where the first character in the needle
// can be found throughout the haystack
func BenchmarkFindAllIndexManyPartialMatchesCaseInsensitive(b *testing.B) {
	r := regexp.MustCompile(`(?i)1test`)
	haystack := []byte(testMatchEndCase)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 1, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseManyPartialMatchesCaseInsensitive(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(testMatchEndCase, "1test")
		assert.Equal(b, 1, iter.Count(matches))
	}
}

// This benchmark simulates a bad case of there being many
// partial matches where the first character in the needle
// can be found throughout the haystack
func BenchmarkFindAllIndexUnicodeManyPartialMatchesCaseInsensitive(b *testing.B) {
	r := regexp.MustCompile(`(?i)Ⱥtest`)
	haystack := []byte(testUnicodeMatchEndCase)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 1, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseUnicodeManyPartialMatchesCaseInsensitive(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(testUnicodeMatchEndCase, "Ⱥtest")
		assert.Equal(b, 1, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexUnicodeCaseInsensitiveVeryLarge(b *testing.B) {
	r := regexp.MustCompile(`(?i)Ⱥtest`)
	haystack := []byte(_large101)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 101, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseUnicodeCaseInsensitiveVeryLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large101, "Ⱥtest")
		assert.Equal(b, 101, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveVeryLarge(b *testing.B) {
	r := regexp.MustCompile(`(?i)ſ`)
	haystack := []byte(_large101)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 101, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveVeryLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large101, "ſ")
		assert.Equal(b, 101, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle1(b *testing.B) {
	r := regexp.MustCompile(`(?i)a`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "a")
		assert.Equal(b, 0, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle2(b *testing.B) {
	r := regexp.MustCompile(`(?i)aa`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "aa")
		assert.Equal(b, 0, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle3(b *testing.B) {
	r := regexp.MustCompile(`(?i)aaa`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "aaa")
		assert.Equal(b, 0, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle4(b *testing.B) {
	r := regexp.MustCompile(`(?i)aaaa`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "aaaa")
		assert.Equal(b, 0, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle5(b *testing.B) {
	r := regexp.MustCompile(`(?i)aaaaa`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle5(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "aaaaa")
		assert.Equal(b, 0, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle6(b *testing.B) {
	r := regexp.MustCompile(`(?i)aaaaaa`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle6(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "aaaaaa")
		assert.Equal(b, 0, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle7(b *testing.B) {
	r := regexp.MustCompile(`(?i)aaaaaaa`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle7(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "aaaaaaa")
		assert.Equal(b, 0, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle8(b *testing.B) {
	r := regexp.MustCompile(`(?i)aaaaaaaa`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle8(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "aaaaaaaa")
		assert.Equal(b, 0, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle9(b *testing.B) {
	r := regexp.MustCompile(`(?i)aaaaaaaaa`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle9(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "aaaaaaaaa")
		assert.Equal(b, 0, iter.Count(matches))
	}
}

func BenchmarkFindAllIndexFoldingCaseInsensitiveNeedle10(b *testing.B) {
	r := regexp.MustCompile(`(?i)aaaaaaaaaa`)
	haystack := []byte(_large11)

	for i := 0; i < b.N; i++ {
		matches := r.FindAllIndex(haystack, -1)
		assert.Equal(b, 0, len(matches))
	}
}

func BenchmarkIndexesAllIgnoreCaseFoldingCaseInsensitiveNeedle10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matches := IndexAllIgnoreCase(_large11, "aaaaaaaaaa")
		assert.Equal(b, 0, iter.Count(matches))
	}
}
