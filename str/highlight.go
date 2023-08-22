package str

import (
	"strings"
)

// HighlightString takes in some content and locations and then inserts in/out
// strings which can be used for highlighting around matching terms. For example
// you could pass in "test" and have it return "<strong>te</strong>st"
// locations accepts output from regex.FindAllIndex IndexAllIgnoreCase or IndexAll
func HighlightString(content string, locations [][2]int, in, out string) string {
	var str strings.Builder

	end := -1
	var found bool

	// Profiles show that most time is spent looking against the locations
	// so we generated a cache quickly to speed that process up
	highlightCache := map[int]struct{}{}
	for _, val := range locations {
		highlightCache[val[0]] = struct{}{}
	}

	// Range over str which is rune aware so even if we get invalid
	// locations we should hopefully ignore them as the byte offset wont
	// match
	for i, x := range content {
		found = false

		// Find which of the locations match
		// and if so write the start str
		if _, ok := highlightCache[i]; ok {
			for _, location := range locations {
				if i != location[0] {
					continue
				}
				// We have a match where the outer index matches
				// against the location[0] which contains the location of the match

				// We only write the found str once per match and
				// only if we are not in the middle of one
				if !found && end <= 0 {
					str.WriteString(in)
					found = true
				}

				// Determine the expected end location for this match
				// and only if its further than the expected end do we
				// change to deal with overlaps if say we are trying to match
				// on t and tes against test where we want tes as the longest
				// match to be the end that's written
				end = max(end, location[1]-1) // location[1] in this case is the length of the match
			}
		}

		// This deals with characters that are multi-byte and as such we never range over
		// the rest and as such we need to remember to close them off if we have gone past
		// their end. As such this needs to come before we write the current byte
		if end > 0 && i > end {
			str.WriteString(out)
			end = 0
		}

		str.WriteRune(x)

		// If at the end, and its not -1 meaning the first char
		// which should never happen (I hope!) then write the end str
		if i == end && end != -1 {
			str.WriteString(out)
			end = 0
		}
	}

	return str.String()
}
