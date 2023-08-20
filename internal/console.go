package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/rprtr258/cs/str"
)

func NewConsoleSearch() {
	files := FindFiles(strings.Join(SearchString, " "))
	toProcessQueue := make(chan *FileJob, runtime.NumCPU()) // Files to be read into memory for processing
	summaryQueue := make(chan *FileJob, runtime.NumCPU())   // Files that match and need to be displayed

	// parse the query here to get the fuzzy stuff
	query, fuzzy := PreParseQuery(SearchString)
	fileReaderWorker := NewFileReaderWorker(files, toProcessQueue, fuzzy)

	fileSearcher := NewSearcherWorker(toProcessQueue, summaryQueue, query)

	resultSummarizer := NewResultSummarizer(summaryQueue, fileReaderWorker, SnippetCount)

	go fileReaderWorker.Start()
	go fileSearcher.Start()
	resultSummarizer.Start()
}

type ResultSummarizer struct {
	input            chan *FileJob
	ResultLimit      int64
	FileReaderWorker *FileReaderWorker
	SnippetCount     int64
	NoColor          bool
	Format           string
	FileOutput       string
}

func NewResultSummarizer(input chan *FileJob, fileReaderWorker *FileReaderWorker, SnippetCount int64) ResultSummarizer {
	return ResultSummarizer{
		input:            input,
		ResultLimit:      -1,
		SnippetCount:     SnippetCount,
		NoColor:          os.Getenv("TERM") == "dumb" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())),
		Format:           Format,
		FileOutput:       FileOutput,
		FileReaderWorker: fileReaderWorker,
	}
}

func (f *ResultSummarizer) Start() {
	// First step is to collect results so we can rank them
	results := []*FileJob{}
	for res := range f.input {
		results = append(results, res)
	}

	// Consider moving this check into processor to save on CPU burn there at some point in
	// the future
	if f.ResultLimit != -1 {
		if int64(len(results)) > f.ResultLimit {
			results = results[:f.ResultLimit]
		}
	}

	rankResults(int(f.FileReaderWorker.GetFileCount()), results)

	switch f.Format {
	case "json":
		f.formatJson(results)
	case "vimgrep":
		f.formatVimGrep(results)
	default:
		f.formatDefault(results)
	}
}

func (f *ResultSummarizer) formatVimGrep(results []*FileJob) {
	var vimGrepOutput []string
	SnippetLength = 50 // vim quickfix puts each hit on its own line.
	documentFrequency := calculateDocumentTermFrequency(results)

	// Cycle through files with matches and process each snippets inside it.
	for _, res := range results {
		snippets := extractRelevantV3(res, documentFrequency, int(SnippetLength))
		if int64(len(snippets)) > f.SnippetCount {
			snippets = snippets[:f.SnippetCount]
		}

		for _, snip := range snippets {
			hint := strings.ReplaceAll(snip.Content, "\n", "\\n")
			line := fmt.Sprintf("%v:%v:%v:%v", res.Location, snip.LinePos[0], snip.Pos[0], hint)
			vimGrepOutput = append(vimGrepOutput, line)
		}
	}

	printable := strings.Join(vimGrepOutput, "\n")
	fmt.Println(printable)
}

func (f *ResultSummarizer) formatJson(results []*FileJob) {
	var jsonResults []jsonResult

	documentFrequency := calculateDocumentTermFrequency(results)

	for _, res := range results {
		v3 := extractRelevantV3(res, documentFrequency, int(SnippetLength))[0]

		// We have the snippet so now we need to highlight it
		// we get all the locations that fall in the snippet length
		// and then remove the length of the snippet cut which
		// makes out location line up with the snippet size
		var l [][2]int
		for _, value := range res.MatchLocations {
			for _, s := range value {
				if s[0] >= v3.Pos[0] && s[1] <= v3.Pos[1] {
					l = append(l, [2]int{
						s[0] - v3.Pos[0],
						s[1] - v3.Pos[0],
					})
				}
			}
		}

		jsonResults = append(jsonResults, jsonResult{
			Filename:       res.Filename,
			Location:       res.Location,
			Content:        v3.Content,
			Score:          res.Score,
			MatchLocations: l,
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

	documentFrequency := calculateDocumentTermFrequency(results)

	for _, res := range results {
		snippets := extractRelevantV3(res, documentFrequency, int(SnippetLength))
		if int64(len(snippets)) > f.SnippetCount {
			snippets = snippets[:f.SnippetCount]
		}

		lines := ""
		for i := 0; i < len(snippets); i++ {
			lines += fmt.Sprintf("%d-%d ", snippets[i].LinePos[0], snippets[i].LinePos[1])
		}

		color.Magenta(fmt.Sprintf("%s Lines %s(%.3f)", res.Location, lines, res.Score))

		for i := 0; i < len(snippets); i++ {
			// We have the snippet so now we need to highlight it
			// we get all the locations that fall in the snippet length
			// and then remove the length of the snippet cut which
			// makes out location line up with the snippet size
			var l [][2]int
			for _, value := range res.MatchLocations {
				for _, s := range value {
					if s[0] >= snippets[i].Pos[0] && s[1] <= snippets[i].Pos[1] {
						l = append(l, [2]int{
							s[0] - snippets[i].Pos[0],
							s[1] - snippets[i].Pos[0],
						})
					}
				}
			}

			displayContent := snippets[i].Content

			// If the start and end pos are 0 then we don't need to highlight because there is
			// nothing to do so, which means its likely to be a filename match with no content
			if !(snippets[i].Pos[0] == 0 && snippets[i].Pos[1] == 0) {
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

type jsonResult struct {
	Filename       string   `json:"filename"`
	Location       string   `json:"location"`
	Content        string   `json:"content"`
	Score          float64  `json:"score"`
	MatchLocations [][2]int `json:"matchlocations"`
}
