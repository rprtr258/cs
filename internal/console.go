package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/rprtr258/fun/iter"

	"github.com/boyter/cs/str"
)

func NewConsoleSearch(SearchString []string) {
	files := FindFiles()

	// parse the query here to get the fuzzy stuff
	query, fuzzy := PreParseQuery(SearchString)

	fileReaderWorker := NewFileReaderWorker(fuzzy)
	toProcessQueue := fileReaderWorker.Start(files)

	summaryQueue := NewSearcherWorker(query).Start(toProcessQueue)

	resultSummarizer := NewResultSummarizer()
	resultSummarizer.FileReaderWorker = fileReaderWorker
	resultSummarizer.SnippetCount = SnippetCount
	resultSummarizer.Start(summaryQueue)
}

type ResultSummarizer struct {
	ResultLimit      int64
	FileReaderWorker *FileReaderWorker
	SnippetCount     int
	NoColor          bool
	Format           string
	FileOutput       string
}

func NewResultSummarizer() ResultSummarizer {
	return ResultSummarizer{
		ResultLimit:  -1,
		SnippetCount: 1,
		NoColor:      os.Getenv("TERM") == "dumb" || !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()),
		Format:       Format,
		FileOutput:   FileOutput,
	}
}

func (f *ResultSummarizer) Start(input chan *FileJob) {
	// First step is to collect results so we can rank them
	results := []*FileJob{}
	for res := range input {
		results = append(results, res)
	}

	// Consider moving this check into processor to save on CPU burn there at some point in
	// the future
	if f.ResultLimit != -1 && int64(len(results)) > f.ResultLimit {
		results = results[:f.ResultLimit]
	}

	rankResults(int(f.FileReaderWorker.GetFileCount()), results)

	switch f.Format {
	case "json":
		f.formatJson(results)
	case "vimgrep":
		f.formatVimGrep(iter.FromMany(results...))
	default:
		f.formatDefault(results)
	}
}

func (f *ResultSummarizer) formatVimGrep(results iter.Seq[*FileJob]) {
	const SnippetLength = 50 // vim quickfix puts each hit on its own line.
	documentFrequency := calculateDocumentTermFrequency(results)

	// Cycle through files with matches and process each snippets inside it.
	vimGrepOutput := iter.FlatMap(
		results,
		func(res *FileJob) iter.Seq[string] {
			return iter.Map(
				extractRelevantV3(res, documentFrequency, SnippetLength).
					Take(f.SnippetCount),
				func(snip Snippet) string {
					return fmt.Sprintf("%s:%d:%d:%s",
						res.Location,
						snip.LineStart,
						snip.Pos[0],
						strings.ReplaceAll(snip.Content, "\n", "\\n"),
					)
				})
		}).ToSlice()

	fmt.Println(strings.Join(vimGrepOutput, "\n"))
}

type jsonResult struct {
	Filename       string   `json:"filename"`
	Location       string   `json:"location"`
	Content        string   `json:"content"`
	Score          float64  `json:"score"`
	MatchLocations [][2]int `json:"matchlocations"`
}

func (f *ResultSummarizer) formatJson(results []*FileJob) {
	documentFrequency := calculateDocumentTermFrequency(iter.FromMany(results...))

	var jsonResults []jsonResult
	for _, res := range results {
		v3, _ := extractRelevantV3(res, documentFrequency, int(SnippetLength)).Head()

		// We have the snippet so now we need to highlight it
		// we get all the locations that fall in the snippet length
		// and then remove the length of the snippet cut which
		// makes out location line up with the snippet size
		jsonResults = append(jsonResults, jsonResult{
			Filename: res.Filename,
			Location: res.Location,
			Content:  v3.Content,
			Score:    res.Score,
			MatchLocations: iter.
				Flatten(iter.Values(iter.FromDict(res.MatchLocations))).
				Filter(func(s [2]int) bool {
					return s[0] >= v3.Pos[0] && s[1] <= v3.Pos[1]
				}).
				Map(func(s [2]int) [2]int {
					return [2]int{s[0] - v3.Pos[0], s[1] - v3.Pos[0]}
				}).
				ToSlice(),
		})
	}

	jsonString, _ := json.Marshal(jsonResults)
	if f.FileOutput == "" {
		fmt.Println(string(jsonString))
	} else {
		_ = os.WriteFile(FileOutput, jsonString, 0o600)
		fmt.Println("results written to " + FileOutput)
	}
}

func (f *ResultSummarizer) formatDefault(results []*FileJob) {
	fmtBegin := "\033[1;31m"
	fmtEnd := "\033[0m"
	if f.NoColor {
		fmtBegin = ""
		fmtEnd = ""
	}

	documentFrequency := calculateDocumentTermFrequency(iter.FromMany(results...))

	for _, res := range results {
		snippets := extractRelevantV3(res, documentFrequency, int(SnippetLength)).
			Take(f.SnippetCount).
			ToSlice()

		lines := ""
		for i := 0; i < len(snippets); i++ {
			lines += fmt.Sprintf("%d-%d ", snippets[i].LineStart, snippets[i].LineEnd)
		}

		color.Magenta(fmt.Sprintf("%s Lines %s(%.3f)", res.Location, lines, res.Score))

		for i := 0; i < len(snippets); i++ {
			// We have the snippet so now we need to highlight it
			// we get all the locations that fall in the snippet length
			// and then remove the length of the snippet cut which
			// makes out location line up with the snippet size
			displayContent := snippets[i].Content

			// If the start and end pos are 0 then we don't need to highlight because there is
			// nothing to do so, which means its likely to be a filename match with no content
			if snippets[i].Pos[0] != 0 || snippets[i].Pos[1] != 0 {
				l := iter.Map(
					iter.Filter(
						iter.Flatten(
							iter.Values(
								iter.FromDict(
									res.MatchLocations))),
						func(s [2]int) bool {
							return s[0] >= snippets[i].Pos[0] && s[1] <= snippets[i].Pos[1]
						}),
					func(s [2]int) [2]int {
						return [2]int{
							s[0] - snippets[i].Pos[0],
							s[1] - snippets[i].Pos[0],
						}
					},
				)

				displayContent = str.HighlightString(snippets[i].Content, l, fmtBegin, fmtEnd)
			}

			fmt.Println(displayContent)
			if i == len(snippets)-1 {
				fmt.Println("")
			} else {
				fmt.Println("")
				fmt.Println("\u001B[1;37m……………snip……………\u001B[0m")
				fmt.Println("")
			}
		}
	}
}
