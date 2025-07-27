package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"

	"github.com/rprtr258/cs/internal/core"
	"github.com/rprtr258/cs/internal/str"
)

func NewConsoleSearch() {
	files := core.FindFiles(strings.Join(core.SearchString, " "))
	toProcessCh := make(chan *core.FileJob, runtime.NumCPU()) // Files to be read into memory for processing
	summaryCh := make(chan *core.FileJob, runtime.NumCPU())   // Files that match and need to be displayed

	// parse the query here to get the fuzzy stuff
	query, fuzzy := core.PreParseQuery(core.SearchString)
	fileReaderWorker := core.NewFileReaderWorker(files, toProcessCh, fuzzy)

	go fileReaderWorker.Start()
	go core.NewSearcherWorker(toProcessCh, summaryCh, query)
	NewResultSummarizer(summaryCh, fileReaderWorker, core.SnippetCount)
}

type ResultSummarizer struct {
	input            chan *core.FileJob
	ResultLimit      int
	FileReaderWorker *core.FileReaderWorker
	SnippetCount     int
	NoColor          bool
	Format           string
	FileOutput       string
}

func NewResultSummarizer(input chan *core.FileJob, fileReaderWorker *core.FileReaderWorker, snippetCount int) {
	f := &ResultSummarizer{
		input:            input,
		ResultLimit:      -1,
		SnippetCount:     snippetCount,
		Format:           core.Format,
		FileOutput:       core.FileOutput,
		FileReaderWorker: fileReaderWorker,
		NoColor: os.Getenv("TERM") == "dumb" ||
			!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()),
	}

	// First step is to collect results so we can rank them
	results := []*core.FileJob{}
	for res := range f.input {
		results = append(results, res)

		// TODO: Consider moving this check into processor to save on CPU burn there at some point in the future
		if f.ResultLimit != -1 && len(results) >= f.ResultLimit {
			break
		}
	}

	core.RankResults(f.FileReaderWorker.GetFileCount(), results)

	switch f.Format {
	case "json":
		f.formatJson(results)
	case "vimgrep":
		f.formatVimGrep(results)
	default:
		f.formatDefault(results)
	}
}

func (f *ResultSummarizer) formatVimGrep(results []*core.FileJob) {
	// TODO: wtf, changing global here is needed?
	core.SnippetLength = 50 // vim quickfix puts each hit on its own line.
	documentFrequency := core.CalculateDocumentTermFrequency(slices.Values(results))

	// Cycle through files with matches and process each snippets inside it.
	var vimGrepOutput []string
	for _, res := range results {
		snippets := slices.Collect(core.ExtractRelevantV3(res, documentFrequency, core.SnippetLength))
		if len(snippets) > f.SnippetCount {
			snippets = snippets[:f.SnippetCount]
		}

		for _, snip := range snippets {
			hint := strings.ReplaceAll(snip.Content, "\n", "\\n")
			line := fmt.Sprintf("%v:%v:%v:%v", res.Location, snip.LinePos[0], snip.Pos[0], hint)
			vimGrepOutput = append(vimGrepOutput, line)
		}
	}
	fmt.Println(strings.Join(vimGrepOutput, "\n"))
}

func (f *ResultSummarizer) formatJson(results []*core.FileJob) {
	documentFrequency := core.CalculateDocumentTermFrequency(slices.Values(results))

	type jsonResult struct {
		Filename       string   `json:"filename"`
		Location       string   `json:"location"`
		Content        string   `json:"content"`
		Score          float64  `json:"score"`
		MatchLocations [][2]int `json:"matchlocations"`
	}
	jsonResults := make([]jsonResult, 0, len(results))
	for _, res := range results {
		v3, _ := first(core.ExtractRelevantV3(res, documentFrequency, core.SnippetLength))

		// We have the snippet so now we need to highlight it
		// we get all the locations that fall in the snippet length
		// and then remove the length of the snippet cut which
		// makes out location line up with the snippet size
		l := slices.Collect(func(yield func([2]int) bool) {
			for _, value := range res.MatchLocations {
				for _, s := range value {
					p := [2]int{
						s[0] - v3.Pos[0],
						s[1] - v3.Pos[1],
					}
					if p[0] >= 0 && p[1] <= 0 && !yield(p) {
						return
					}
				}
			}
		})

		jsonResults = append(jsonResults, jsonResult{
			Filename:       res.Filename,
			Location:       res.Location,
			Content:        v3.Content,
			Score:          res.Score,
			MatchLocations: l,
		})
	}

	var w io.Writer
	if f.FileOutput == "" {
		w = os.Stdout
	} else {
		w, _ = os.OpenFile(core.FileOutput, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		fmt.Println("results written to " + core.FileOutput)
	}
	_ = json.NewEncoder(w).Encode(jsonResults)
}

func (f *ResultSummarizer) formatDefault(results []*core.FileJob) {
	fmtBegin := "\033[1;31m"
	fmtEnd := "\033[0m"
	if f.NoColor {
		fmtBegin = ""
		fmtEnd = ""
	}

	documentFrequency := core.CalculateDocumentTermFrequency(slices.Values(results))

	for _, result := range results {
		snippets := slices.Collect(core.ExtractRelevantV3(result, documentFrequency, core.SnippetLength))
		if len(snippets) > f.SnippetCount {
			snippets = snippets[:f.SnippetCount]
		}

		lines := ""
		for _, snippet := range snippets {
			lines += fmt.Sprintf("%d-%d ", snippet.LinePos[0], snippet.LinePos[1])
		}

		color.Magenta(fmt.Sprintf("%s Lines %s(%.3f)", result.Location, lines, result.Score))

		for i, snippet := range snippets {
			// We have the snippet so now we need to highlight it
			// we get all the locations that fall in the snippet length
			// and then remove the length of the snippet cut which
			// makes out location line up with the snippet size
			l := func(yield func([2]int) bool) {
				for _, value := range result.MatchLocations {
					for _, s := range value {
						if s[0] >= snippet.Pos[0] && s[1] <= snippet.Pos[1] {
							if !yield([2]int{
								s[0] - snippet.Pos[0],
								s[1] - snippet.Pos[0],
							}) {
								return
							}
						}
					}
				}
			}

			displayContent := snippet.Content

			// If the start and end pos are 0 then we don't need to highlight because there is
			// nothing to do so, which means its likely to be a filename match with no content
			if snippet.Pos[0] != 0 || snippet.Pos[1] != 0 {
				displayContent = str.HighlightString(snippet.Content, l, fmtBegin, fmtEnd)
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
