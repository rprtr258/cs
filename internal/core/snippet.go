package core

import (
	"bytes"
	"cmp"
	"iter"
	"slices"
	"unicode"

	"github.com/rprtr258/cs/internal/str"
)

const (
	_snipSideMax int = 10 // Defines the maximum bytes either side of the match we are willing to return
	// The below are used for adding boosts to match conditions of snippets to hopefully produce the best match
	_phraseHeavyBoost = 20
	_spaceBoundBoost  = 5
	_exactMatchBoost  = 5
	// Below is used to control CPU burn time trying to find the most relevant snippet
	_relevanceCutoff = 10_000
)

type bestMatch struct {
	Pos      [2]int
	Score    float64
	Relevant []relevantV3
}

// Internal structure used just for matching things together
type relevantV3 struct {
	Word     string
	Location [2]int
}

type Snippet struct {
	Content string
	Pos     [2]int
	Score   float64
	LinePos [2]int
}

func iverson(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// Looks through the locations using a sliding window style algorithm
// where it "brute forces" the solution by iterating over every location we have
// and look for all matches that fall into the supplied length and ranking
// based on how many we have.
//
// This algorithm ranks using document frequencies that are kept for
// TF/IDF ranking with various other checks. Look though the source
// to see how it actually works as it is a constant work in progress.
// Some examples of what it can produce which I consider good results,
//
// corpus: Jane Austens Pride and Prejudice
// searchtext: ten thousand a year
// result:  before. I hope he will overlook
//
//	it. Dear, dear Lizzy. A house in town! Every thing that is
//	charming! Three daughters married! Ten thousand a year! Oh, Lord!
//	What will become of me. I shall go distracted.”
//
//	This was enough to prove that her approbation need not be
//
// searchtext: poor nerves
// result:  your own children in such a way?
//
//	You take delight in vexing me. You have no compassion for my poor
//	nerves.”
//
//	“You mistake me, my dear. I have a high respect for your nerves.
//	They are my old friends. I have heard you mention them with
//	consideration these last
//
// The above are captured in the tests for this method along with extractions from rhyme of the ancient mariner
// and generally we do not want them to regress for any other gains.
//
// Please note that testing this is... hard. This is because what is considered relevant also happens
// to differ between people. Heck a few times I have been disappointed with results that I was previously happy with.
// As such this is not tested as much as other methods and you should not rely on the results being static over time
// as the internals will be modified to produce better results where possible
func ExtractRelevantV3(res *FileJob, documentFrequencies map[string]int, relLength int) iter.Seq[Snippet] {
	wrapLength := relLength / 2

	rv3 := convertToRelevant(res)
	// if we have a huge amount of matches we want to reduce it because otherwise it takes forever
	// to return something if the search has many matches.
	rv3 = rv3[:min(len(rv3), _relevanceCutoff)]

	var bestMatches []bestMatch
	// Slide around looking for matches that fit in the length
	for i := 0; i < len(rv3); i++ {
		m := bestMatch{
			Pos:      rv3[i].Location,
			Relevant: []relevantV3{rv3[i]},
		}

		// Slide left
		// Ensure we never step outside the bounds of our slice
		for j := i - 1; j >= 0; j-- {
			// How close is the matches start to our end?
			// If the diff is greater than the target then break out as there is no
			// more reason to keep looking as the slice is sorted
			if rv3[i].Location[1]-rv3[j].Location[0] > wrapLength {
				break
			}

			// If we didn't break this is considered a larger match
			m.Pos[0] = rv3[j].Location[0]
			m.Relevant = append(m.Relevant, rv3[j])
		}

		// Slide right
		// Ensure we never step outside the bounds of our slice
		for j := i + 1; j < len(rv3); j++ {
			// How close is the matches end to our start?
			// If the diff is greater than the target then break out as there is no
			// more reason to keep looking as the slice is sorted
			if rv3[j].Location[1]-rv3[i].Location[0] > wrapLength {
				break
			}

			m.Pos[1] = rv3[j].Location[1]
			m.Relevant = append(m.Relevant, rv3[j])
		}

		// If the match around this isn't long enough expand it out
		// roughly based on how large a context we need to add
		l := m.Pos[1] - m.Pos[0]
		if l < relLength {
			add := (relLength - l) / 2
			m.Pos = [2]int{
				max(m.Pos[0]-add, 0),
				min(m.Pos[1]+add, len(res.Content)),
			}
		}

		// Now we see if there are any nearby spaces to avoid us cutting in the
		// middle of a word if we can avoid it
		var sf, ef bool
		m.Pos[0], sf = findSpaceLeft(res, m.Pos[0], _snipSideMax)
		m.Pos[1], ef = findSpaceRight(res, m.Pos[1], _snipSideMax)

		// Check if we are cutting in the middle of a multibyte char and if so
		// go looking till we find the start. We only do so if we didn't find a space,
		// and if we aren't at the start or very end of the content
		for !sf && m.Pos[0] != 0 && m.Pos[0] != len(res.Content) && !str.StartOfRune(res.Content[m.Pos[0]]) {
			m.Pos[0]--
		}
		for !ef && m.Pos[1] != 0 && m.Pos[1] != len(res.Content) && !str.StartOfRune(res.Content[m.Pos[1]]) {
			m.Pos[1]--
		}

		// If we are very close to the start, just push it out so we get the actual start
		if m.Pos[0] <= _snipSideMax {
			m.Pos[0] = 0
		}
		// As above, but against the end so we just include the rest if we are close
		if len(res.Content)-m.Pos[1] <= 10 {
			m.Pos[1] = len(res.Content)
		}

		// Now that we have the snippet start to rank it to produce a score indicating
		// how good a match it is and hopefully display to the user what they
		// were actually looking for
		m.Score += float64(len(m.Relevant)) // Factor in how many matches we have
		// NB the below is commented out because it seems to make things worse generally
		// m.Score += float64(m.Pos[1] - m.Pos[0]) // Factor in how large the snippet is

		// Apply higher score where the words are near each other
		// mid := rv3[i].Start + (rv3[i].End-rv3[i].End)/2 // match word midpoint
		mid := rv3[i].Location[0]
		for _, v := range m.Relevant {
			p := (v.Location[0] + v.Location[1]) / 2 // comparison word midpoint

			// If the word is within a reasonable distance of this word boost the score
			// weighted by how common that word is so that matches like 'a' impact the rank
			// less than something like 'cromulent' which in theory should not occur as much
			m.Score += iverson(abs(mid-p) < relLength/3) * 100 / float64(documentFrequencies[v.Word])
		}

		// Try to make it phrase heavy such that if words line up next to each other
		// it is given a much higher weight
		for _, v := range m.Relevant {
			// Use 2 here because we want to avoid punctuation such that a search for
			// cat dog will still be boosted if we find cat. dog
			m.Score += _phraseHeavyBoost * iverson(
				abs(rv3[i].Location[0]-v.Location[1]) <= 2 ||
					abs(rv3[i].Location[1]-v.Location[0]) <= 2)
		}

		// If the match is bounded by a space boost it slightly
		// because its likely to be a better match
		m.Score += _spaceBoundBoost*iverson(rv3[i].Location[0] >= 1 && unicode.IsSpace(rune(res.Content[rv3[i].Location[0]-1]))) +
			_spaceBoundBoost*iverson(rv3[i].Location[1] < len(res.Content)-1 && unicode.IsSpace(rune(res.Content[rv3[i].Location[1]+1]))) +

			// If the word is an exact match to what the user typed boost it
			// So while the search may be case insensitive the ranking of
			// the snippet does consider case when boosting ever so slightly
			_exactMatchBoost*iverson(string(res.Content[rv3[i].Location[0]:rv3[i].Location[1]]) == rv3[i].Word)

		// This mod applies over the whole score because we want to most unique words to appear in the middle
		// of the snippet over those where it is on the edge which this should achieve even if it means
		// we may miss out on a slightly better match
		m.Score /= float64(documentFrequencies[rv3[i].Word]) // Factor in how unique the word is
		bestMatches = append(bestMatches, m)
	}

	// Sort our matches by score such that tbe best snippets are at the top
	slices.SortFunc(bestMatches, func(i, j bestMatch) int {
		return cmp.Compare(j.Score, i.Score)
	})

	// Now what we have it sorted lets get just the ones that don't overlap so we have all the unique snippets
	var bestMatchesClean []bestMatch
	var ranges [][2]int
	for _, b := range bestMatches {
		isOverlap := func() bool {
			for _, r := range ranges {
				if r[0] <= b.Pos[0] && b.Pos[0] <= r[1] ||
					r[0] <= b.Pos[1] && b.Pos[1] <= r[1] {
					return true
				}
			}
			return false
		}()

		if !isOverlap {
			ranges = append(ranges, b.Pos)
			bestMatchesClean = append(bestMatchesClean, b)
		}
	}

	// Limit to the 20 best matches
	bestMatchesClean = bestMatchesClean[:min(len(bestMatchesClean), 20)]

	return func(yield func(Snippet) bool) {
		for _, b := range bestMatchesClean {
			match := res.Content[b.Pos[0]:b.Pos[1]]

			index := bytes.Index(res.Content, match)

			startLineOffset := 1
			for i := 0; i < index; i++ {
				if res.Content[i] == '\n' {
					startLineOffset++
				}
			}

			if !yield(Snippet{
				Content: string(match),
				Pos:     b.Pos,
				Score:   b.Score,
				LinePos: [2]int{
					startLineOffset,
					startLineOffset + bytes.Count(match, []byte{'\n'}),
				},
			}) {
				return
			}
		}
	}
}

// Get all of the locations into a new data structure
// which makes things easy to sort and deal with
func convertToRelevant(res *FileJob) []relevantV3 {
	var rv3 []relevantV3
	for word, locations := range res.MatchLocations {
		for _, loc := range locations {
			rv3 = append(rv3, relevantV3{
				Word:     word,
				Location: loc,
			})
		}
	}

	// Sort the results so when we slide around everything is in order
	slices.SortFunc(rv3, func(i, j relevantV3) int {
		return cmp.Compare(i.Location[0], j.Location[0])
	})

	return rv3
}

// Looks for a nearby whitespace character near this position (`pos`)
// up to `distance` away.  Returns index of space if a space was found and
// true, otherwise returns the original index and false
func findSpaceRight(res *FileJob, pos, distance int) (int, bool) {
	if len(res.Content) == 0 {
		return pos, false
	}

	end := min(pos+distance, len(res.Content)-1)

	// Look for spaces
	for i := pos; i <= end; i++ {
		if str.StartOfRune(res.Content[i]) && unicode.IsSpace(rune(res.Content[i])) {
			return i, true
		}
	}

	return pos, false
}

// Looks for nearby whitespace character near this position
// up to distance away. Returns index of space if a space was found and tru
// otherwise the original index is return and false
func findSpaceLeft(res *FileJob, pos, distance int) (int, bool) {
	if len(res.Content) == 0 || pos >= len(res.Content) {
		return pos, false
	}

	// Look for spaces
	for i := pos; i >= max(0, pos-distance); i-- {
		if str.StartOfRune(res.Content[i]) && unicode.IsSpace(rune(res.Content[i])) {
			return i, true
		}
	}

	return pos, false
}

// abs returns the absolute value of x.
func abs(x int) int {
	return max(x, -x)
}
