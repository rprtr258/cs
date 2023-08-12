package internal

import (
	"strings"

	"github.com/boyter/cs/str"
	"github.com/rprtr258/fun"
	"github.com/rprtr258/fun/iter"
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

// PreParseQuery pulls out the file: syntax that we support for fuzzy matching
// where we filter filenames based on this term
func PreParseQuery(args []string) ([]string, string) {
	modified := []string{}
	fuzzy := ""

	for _, s := range args {
		ls := strings.TrimSpace(strings.ToLower(s))
		switch {
		case strings.HasPrefix(ls, "file:"):
			fuzzy = strings.TrimPrefix(ls, "file:")
		case strings.HasPrefix(ls, "filename:"):
			fuzzy = strings.TrimPrefix(ls, "filename:")
		default:
			modified = append(modified, s)
		}
	}

	return modified, fun.Pipe(fuzzy,
		strings.TrimSpace,
		strings.ToLower,
		strings.TrimSpace,
	)
}

// ParseQuery is a cheap and nasty parser. Needs to be reworked
// to provide real boolean logic with AND OR NOT
// but does enough for now
func ParseQuery(args []string) []searchParams {
	// Clean the arguments to avoid redundant spaces and the like
	cleanArgs := iter.
		FromMany(args...).
		Map(strings.TrimSpace).
		ToSlice()

	params := []searchParams{}
	startIndex := 0
	mode := Default

	// With the arguments cleaned up parse out what we need
	// note that this is very ugly
	for ind, arg := range cleanArgs {
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
					startIndex = ind
				}
			}
		case mode == Quoted && strings.HasSuffix(arg, `"`):
			t := strings.Join(cleanArgs[startIndex:ind+1], " ")
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
					startIndex = ind
				}
			}
		case mode == Regex && strings.HasSuffix(arg, `/`):
			t := strings.Join(cleanArgs[startIndex:ind+1], " ")
			params = append(params, searchParams{
				Term: t[1 : len(t)-1],
				Type: Regex,
			})
			mode = Default
		case arg == "NOT":
			// If we start with NOT we cannot negate so ignore
			if ind != 0 {
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
	if mode == Regex {
		t := strings.Join(cleanArgs[startIndex:], " ")
		params = append(params, searchParams{
			Term: t[1:],
			Type: Regex,
		})
	}
	if mode == Quoted {
		t := strings.Join(cleanArgs[startIndex:], " ")
		params = append(params, searchParams{
			Term: t[1:],
			Type: Quoted,
		})
	}

	return params
}

const _letterDigitFuzzyBytes = `abcdefghijklmnopqrstuvwxyz1234567890`

// Takes in a term and returns a slice of them which contains all the
// fuzzy versions of that str with things such as mis-spellings
// somewhat based on https://norvig.com/spell-correct.html
func makeFuzzyDistanceOne(term string) iter.Seq[string] {
	return str.RemoveStringDuplicates(fun.If(
		len(term) <= 2,
		iter.FromMany(term),
		iter.Concat(
			iter.FromMany(term),
			// Delete letters so turn "test" into "est" "tst" "tet"
			iter.Map(
				iter.FromRange(0, len(term), 1),
				func(i int) string {
					return term[:i] + term[i+1:]
				}),
			// Replace a letter or digit which effectively does transpose for us
			iter.FlatMap(
				iter.FromRange(0, len(term), 1),
				func(i int) iter.Seq[string] {
					return iter.Map(
						iter.Values(
							iter.FromString(
								_letterDigitFuzzyBytes)),
						func(b rune) string {
							return term[:i] + string(b) + term[i+1:]
						})
				}),
			// Insert a letter or digit
			iter.FlatMap(
				iter.FromRange(0, len(term), 1),
				func(i int) iter.Seq[string] {
					return iter.Map(
						iter.Values(
							iter.FromString(
								_letterDigitFuzzyBytes)),
						func(b rune) string {
							return term[:i] + string(b) + term[i:]
						})
				})),
	))
}

// Similar to fuzzy 1 but in this case we add letters
// to make the distance larger
func makeFuzzyDistanceTwo(term string) iter.Seq[string] {
	return str.RemoveStringDuplicates(
		iter.Concat(
			makeFuzzyDistanceOne(term),
			// Maybe they forgot to type a letter? Try adding one
			iter.FlatMap(
				iter.FromRange(0, len(term)+1, 1),
				func(i int) iter.Seq[string] {
					return iter.Map(
						iter.Values(
							iter.FromString(
								_letterDigitFuzzyBytes)),
						func(b rune) string {
							return term[:i] + string(b) + term[i:]
						})
				})))
}
