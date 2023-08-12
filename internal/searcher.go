package internal

import (
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/rprtr258/fun/iter"

	"github.com/boyter/cs/str"
)

type SearcherWorker struct {
	searchParams []searchParams
}

func NewSearcherWorker(query []string) *SearcherWorker {
	return &SearcherWorker{
		// Build out the search params
		searchParams: ParseQuery(query),
	}
}

// Does the actual processing of stats and as such contains the hot path CPU call
func (f *SearcherWorker) Start(input chan *FileJob) chan *FileJob {
	var wg sync.WaitGroup
	output := make(chan *FileJob, runtime.NumCPU()) // Files to be read into memory for processing
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for res := range input {
				// Now we do the actual search against the file
				for i, needle := range f.searchParams {
					didSearch := false
					switch needle.Type {
					case Default, Quoted:
						didSearch = true
						res.MatchLocations[needle.Term] = str.IndexAllIgnoreCase(string(res.Content), needle.Term)
					case Regex:
						if re, err := regexp.Compile(needle.Term); err != nil { // Error indicates a regex compile fail so safe to ignore here
							// Its possible the user supplies an invalid regex and if so we should not crash
							// but ignore it
							// return nil, fmt.Errorf("regex compile failure issue: %w", err)
						} else {
							didSearch = true
							res.MatchLocations[needle.Term] = iter.Map(
								iter.FromMany(re.FindAllIndex(res.Content, -1)...),
								func(match []int) [2]int {
									return [2]int{match[0], match[1]}
								})
						}
					case Fuzzy1:
						didSearch = true
						terms := makeFuzzyDistanceOne(strings.TrimRight(needle.Term, "~1"))
						res.MatchLocations[needle.Term] = iter.FlatMap(
							terms,
							func(t string) iter.Seq[[2]int] {
								return str.IndexAllIgnoreCase(string(res.Content), t)
							})
					case Fuzzy2:
						didSearch = true
						terms := makeFuzzyDistanceTwo(strings.TrimRight(needle.Term, "~2"))
						res.MatchLocations[needle.Term] = iter.FlatMap(
							terms,
							func(t string) iter.Seq[[2]int] {
								return str.IndexAllIgnoreCase(string(res.Content), t)
							})
					}

					// We currently ignore things such as NOT and as such
					// we don't want to break out if we run into them
					// so only update the score IF there was a search
					// which also makes this by default an AND search
					if didSearch {
						count := iter.Count(res.MatchLocations[needle.Term])
						// If we did a search but the previous was a NOT we need to only continue if we found nothing
						if i != 0 && f.searchParams[i-1].Type == Negated {
							if count > 0 {
								res.Score = 0
								break
							}
						} else if count == 0 { // Normal search so ensure we got something by default AND logic rules
							res.Score = 0
							break
						}

						// Without ranking this score favors the most matches which is
						// basic but better than nothing NB this is almost always
						// overridden inside the actual ranker so its only here in case
						// we ever forget that so we at least get something
						res.Score += float64(count)
					}
				}

				if res.Score != 0 {
					output <- res
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}
