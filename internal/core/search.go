package core

import (
	"iter"
	"strings"

	"github.com/rprtr258/cs/internal/str"
)

const (
	Default = iota
	Quoted
	Regex
	Negated
	Fuzzy1
	Fuzzy2
)

type searchParams struct {
	Term string
	Type int64
}

func strt(
	s string,
	fs ...func(string) string,
) string {
	for _, f := range fs {
		s = f(s)
	}
	return s
}

var _queryReplacer = strings.NewReplacer(
	"file:", "",
	"filename:", "",
)

// PreParseQuery pulls out the file: syntax that we support for fuzzy matching
// where we filter filenames based on this term
func PreParseQuery(args []string) ([]string, string) {
	modified := []string{}
	fuzzy := ""
	for _, s := range args {
		if ls := strings.TrimSpace(strings.ToLower(s)); strings.HasPrefix(ls, "file:") || strings.HasPrefix(ls, "filename:") {
			fuzzy = strt(
				ls,
				_queryReplacer.Replace,
				strings.ToLower,
				strings.TrimSpace,
			)
		} else {
			modified = append(modified, s)
		}
	}
	return modified, fuzzy
}

// ParseQuery is a cheap and nasty parser. Needs to be reworked
// to provide real boolean logic with AND OR NOT
// but does enough for now
func ParseQuery(args []string) []searchParams {
	// Clean the arguments to avoid redundant spaces and the like
	cleanArgs := make([]string, len(args))
	for i, arg := range args {
		cleanArgs[i] = strings.TrimSpace(arg)
	}

	// With the arguments cleaned up parse out what we need
	// note that this is very ugly
	params := []searchParams{}
	startIndex := 0
	mode := Default
	for i, arg := range cleanArgs {
		switch {
		case strings.HasPrefix(arg, `"`):
			if len(arg) != 1 {
				if strings.HasSuffix(arg, `"`) {
					params = append(params, searchParams{
						Term: arg[1 : len(arg)-1],
						Type: Quoted,
					})
				} else {
					mode = Quoted
					startIndex = i
				}
			}
		case mode == Quoted && strings.HasSuffix(arg, `"`):
			t := strings.Join(cleanArgs[startIndex:i+1], " ")
			params = append(params, searchParams{
				Term: t[1 : len(t)-1],
				Type: Quoted,
			})
			mode = Default
		case strings.HasPrefix(arg, `/`):
			if len(arg) != 1 {
				// If we end with / not prefixed with a \ we are done
				if strings.HasSuffix(arg, `/`) {
					// If the term is // don't treat it as a regex treat it as a search for //
					if arg == "//" {
						params = append(params, searchParams{
							Term: "//",
							Type: Default,
						})
					} else {
						params = append(params, searchParams{
							Term: arg[1 : len(arg)-1],
							Type: Regex,
						})
					}
				} else {
					mode = Regex
					startIndex = i
				}
			}
		case mode == Regex && strings.HasSuffix(arg, `/`):
			t := strings.Join(cleanArgs[startIndex:i+1], " ")
			params = append(params, searchParams{
				Term: t[1 : len(t)-1],
				Type: Regex,
			})
			mode = Default
		case arg == "NOT":
			// If we start with NOT we cannot negate so ignore
			if i != 0 {
				params = append(params, searchParams{
					Term: arg,
					Type: Negated,
				})
			}
		case strings.HasSuffix(arg, "~1"):
			params = append(params, searchParams{
				Term: strings.TrimRight(arg, "~1"),
				Type: Fuzzy1,
			})
		case strings.HasSuffix(arg, "~2"):
			params = append(params, searchParams{
				Term: strings.TrimRight(arg, "~2"),
				Type: Fuzzy2,
			})
		default:
			params = append(params, searchParams{
				Term: arg,
				Type: Default,
			})
		}
	}

	// If the user didn't end properly that's ok lets do it for them
	switch mode {
	case Regex, Quoted:
		t := strings.Join(cleanArgs[startIndex:], " ")
		params = append(params, searchParams{
			Term: t[1:],
			Type: int64(mode),
		})
	}

	return params
}

const _letterDigitFuzzyBytes = `abcdefghijklmnopqrstuvwxyz1234567890`

// Takes in a term and returns a slice of them which contains all the
// fuzzy versions of that str with things such as mis-spellings
// somewhat based on https://norvig.com/spell-correct.html
func makeFuzzyDistanceOne(term string) iter.Seq[string] {
	return str.RemoveStringDuplicates(func(yield func(string) bool) {
		if !yield(term) || len(term) <= 2 {
			return
		}

		// Delete letters so turn "test" into "est" "tst" "tet"
		for i := range len(term) {
			if !yield(term[:i] + term[i+1:]) {
				return
			}
		}

		// Replace a letter or digit which effectively does transpose for us
		for i := range len(term) {
			for _, b := range _letterDigitFuzzyBytes {
				if !yield(term[:i] + string(b) + term[i+1:]) {
					return
				}
			}
		}

		// Insert a letter or digit
		for i := range len(term) {
			for _, b := range _letterDigitFuzzyBytes {
				if !yield(term[:i] + string(b) + term[i:]) {
					return
				}
			}
		}
	})
}

// Similar to fuzzy 1 but in this case we add letters
// to make the distance larger
func makeFuzzyDistanceTwo(term string) iter.Seq[string] {
	return str.RemoveStringDuplicates(func(yield func(string) bool) {
		for v := range makeFuzzyDistanceOne(term) {
			if !yield(v) {
				return
			}
		}

		// Maybe they forgot to type a letter? Try adding one
		for i := range len(term) + 1 {
			for _, b := range _letterDigitFuzzyBytes {
				if !yield(term[:i] + string(b) + term[i:]) {
					return
				}
			}
		}
	})
}
