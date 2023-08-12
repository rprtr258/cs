package str

import (
	"cmp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/rprtr258/fun"
	"github.com/rprtr258/fun/iter"
)

func cmp2[T cmp.Ordered](a, b [2]T) int {
	return fun.If(
		a[0] != b[0],
		cmp.Compare(a[0], b[0]),
		cmp.Compare(a[1], b[1]),
	)
}

// IndexAll extracts all of the locations of a string inside another string
// up-to the defined limit and does so without regular expressions
// which makes it faster than FindAllIndex in most situations while
// not being any slower. It performs worst when working against random
// data.
//
// Some benchmark results to illustrate the point (find more in index_benchmark_test.go)
//
// BenchmarkFindAllIndex-8                         2458844	       480.0 ns/op
// BenchmarkIndexAll-8                            14819680	        79.6 ns/op
//
// For pure literal searches IE no regular expression logic this method
// is a drop in replacement for re.FindAllIndex but generally much faster.
//
// Similar to how FindAllIndex the limit option can be passed -1
// to get all matches.
//
// Note that this method is explicitly case sensitive in its matching.
// A return value of nil indicates no match.
func IndexAll(haystack, needle string) iter.Seq[[2]int] {
	return func(yield func([2]int) bool) bool {
		// The below needed to avoid timeout crash found using go-fuzz
		if len(haystack) == 0 || len(needle) == 0 {
			return true
		}

		// Perform the first search outside the main loop to make the method
		// easier to understand
		searchText := haystack
		offSet := 0

		// strings.Index does checks of if the string is empty so we don't need
		// to explicitly do it ourselves
		for loc, searchText, ok := strings.Cut(searchText, needle); ok; loc, searchText, ok = strings.Cut(searchText, needle) {
			// yield location of the match in bytes and end location in bytes of the match
			if !yield([2]int{
				len(loc) + offSet,
				len(loc) + offSet + len(needle),
			}) {
				return false
			}

			// We need to keep the offset of the match so we continue searching
			offSet += len(loc) + len(needle)
		}

		return true
	}
}

// if the IndexAllIgnoreCase method is called frequently with the same patterns
// (which is a common case) this is here to speed up the case permutations
// it is limited to a size of 10 so it never gets that large but really
// allows things to run faster
var (
	_permuteCache     = map[string][]string{}
	_permuteCacheLock = sync.Mutex{}
)

// CacheSize this is public so it can be modified depending on project needs
// you can increase this value to cache more of the case permutations which
// can improve performance if doing the same searches over and over
var CacheSize = 10

// IndexAllIgnoreCase extracts all of the locations of a string inside another string
// up-to the defined limit. It is designed to be faster than uses of FindAllIndex with
// case insensitive matching enabled, by looking for string literals first and then
// checking for exact matches. It also does so in a unicode aware way such that a search
// for S will search for S s and ſ which a simple strings.ToLower over the haystack
// and the needle will not.
//
// The result is the ability to search for literals without hitting the regex engine
// which can at times be horribly slow. This by contrast is much faster. See
// index_ignorecase_benchmark_test.go for some head to head results. Generally
// so long as we aren't dealing with random data this method should be considerably
// faster (in some cases thousands of times) or just as fast. Of course it cannot
// do regular expressions, but that's fine.
//
// For pure literal searches IE no regular expression logic this method
// is a drop in replacement for re.FindAllIndex but generally much faster.
func IndexAllIgnoreCase(haystack, needle string) iter.Seq[[2]int] {
	// The below needed to avoid timeout crash found using go-fuzz
	if len(haystack) == 0 || len(needle) == 0 {
		return iter.FromNothing[[2]int]()
	}

	// One of the problems with finding locations ignoring case is that
	// the different case representations can have different byte counts
	// which means the locations using strings or bytes Index can be off
	// if you apply strings.ToLower to your haystack then use strings.Index.
	//
	// This can be overcome using regular expressions but suffers the penalty
	// of hitting the regex engine and paying the price of case
	// insensitive match there.
	//
	// This method tries something else which is used by some regex engines
	// such as the one in Rust where given a str literal if you get
	// all the case options of that such as turning foo into foo Foo fOo FOo foO FoO fOO FOO
	// and then use Boyer-Moore or some such for those. Of course using something
	// like Aho-Corasick or Rabin-Karp to get multi match would be a better idea so you
	// can match all of the input in one pass.
	//
	// If the needle is over some amount of characters long you chop off the first few
	// and then search for those. However this means you are not finding actual matches and as such
	// you the need to validate a potential match after you have found one.
	// The confirmation match is done in a loop because for some literals regular expression
	// is still to slow, although for most its a valid option.

	// Char limit is the cut-off where we switch from all case permutations
	// to just the first 3 and then check for an actual match
	// in my tests 3 speeds things up the most against test data
	// of many famous books concatenated together and large
	// amounts of data from /dev/urandom
	const charLimit = 3

	if utf8.RuneCountInString(needle) <= charLimit {
		// We are below the limit we set, so get all the search
		// terms and search for that

		// Generally speaking I am against caches inside libraries but in this case...
		// when the IndexAllIgnoreCase method is called repeatedly it quite often
		// ends up performing case folding on the same thing over and over again which
		// can become the most expensive operation. So we keep a VERY small cache
		// to avoid that being an issue.
		_permuteCacheLock.Lock()
		searchTerms, ok := _permuteCache[needle]
		if !ok {
			if len(_permuteCache) > CacheSize {
				_permuteCache = map[string][]string{}
			}
			searchTerms = iter.ToSlice(PermuteCaseFolding(needle))
			_permuteCache[needle] = searchTerms
		}
		_permuteCacheLock.Unlock()

		// This is using IndexAll in a loop which was faster than
		// any implementation of Aho-Corasick or Boyer-Moore I tried
		// but in theory Aho-Corasick / Rabin-Karp or even a modified
		// version of Boyer-Moore should be faster than this.
		// Especially since they should be able to do multiple comparisons
		// at the same time.
		// However after some investigation it turns out that this turns
		// into a fancy  vector instruction on AMD64 (which is all we care about)
		// and as such its pretty hard to beat.
		res := iter.FromNothing[[2]int]()
		for _, term := range searchTerms {
			res = iter.MergeFunc(res, IndexAll(haystack, term), cmp2)
		}
		return res
	}

	// Over the character limit so look for potential matches and only then check to find real ones

	// Note that we have to use runes here to avoid cutting bytes off so
	// cast things around to ensure it works

	// Generally speaking I am against caches inside libraries but in this case...
	// when the IndexAllIgnoreCase method is called repeatedly it quite often
	// ends up performing case folding on the same thing over and over again which
	// can become the most expensive operation. So we keep a VERY small cache
	// to avoid that being an issue.
	_permuteCacheLock.Lock()
	needleRunesCount := utf8.RuneCountInString(needle)
	a, aa := utf8.DecodeRuneInString(needle)
	b, bb := utf8.DecodeRuneInString(needle[aa:])
	c, _ := utf8.DecodeRuneInString(needle[bb:])
	s := string([]rune{a, b, c})
	searchTerms, ok := _permuteCache[s]
	if !ok {
		if len(_permuteCache) > CacheSize {
			_permuteCache = map[string][]string{}
		}
		searchTerms = iter.ToSlice(PermuteCaseFolding(s))
		_permuteCache[s] = searchTerms
	}
	_permuteCacheLock.Unlock()

	// This is using IndexAll in a loop which was faster than
	// any implementation of Aho-Corasick or Boyer-Moore I tried
	// but in theory Aho-Corasick / Rabin-Karp or even a modified
	// version of Boyer-Moore should be faster than this.
	// Especially since they should be able to do multiple comparisons
	// at the same time.
	// However after some investigation it turns out that this turns
	// into a fancy  vector instruction on AMD64 (which is all we care about)
	// and as such its pretty hard to beat.

	res := iter.FromNothing[[2]int]()
	for _, term := range searchTerms {
		add := IndexAll(haystack, term).
			MapFilter(func(match [2]int) ([2]int, bool) {
				// We have a potential match, so now see if it actually matches
				// by getting the actual value out of our haystack
				if len(haystack) < match[0]+needleRunesCount {
					return [2]int{}, false
				}

				// Because the length of the needle might be different to what we just found as a match
				// based on byte size we add enough extra on the end to deal with the difference
				e := min(len(needle)+len(needle)-1, len(haystack)-match[0])

				// Cut off the number at the end to the number we need which is the length of the needle runes
				toMatch := []rune(haystack[match[0] : match[0]+e])[:min(e, needleRunesCount)]

				// old logic here
				// toMatch = []rune(haystack[match[0] : match[0]+e])[:len(needleRune)]

				// what we need to do is iterate the runes of the haystack portion we are trying to
				// match and confirm that the same rune position is a actual match or case fold match
				// if they are keep looking, if they are not bail out as its not a real match
				i := 0
				for _, c := range needle {
					if i >= len(toMatch) {
						break
					}
					d := toMatch[i]
					// Check against the actual term and if that's a match we can avoid folding
					// and doing those comparisons to hopefully save some CPU time
					// Not a match so case fold to actually check
					if d != c && !AllSimpleFold(d).
						Any(func(j rune) bool {
							return j == c
						}) {
						return [2]int{}, false
					}
					i++
				}

				// When we have confirmed a match we add it to our total
				// but adjust the positions to the match and the length of the
				// needle to ensure the byte count lines up
				return [2]int{match[0], match[0] + len(string(toMatch))}, true
			})
		res = iter.MergeFunc(res, add, cmp2)
	}
	return res
}
