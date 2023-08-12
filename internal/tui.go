package internal

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boyter/cs/str"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"github.com/rprtr258/fun/iter"
)

type displayResult struct {
	Title      *tview.TextView
	Body       *tview.TextView
	BodyHeight int
	SpacerOne  *tview.TextView
	SpacerTwo  *tview.TextView
	Location   string
}

type codeResult struct {
	Title    string
	Content  string
	Score    float64
	Location string
}

type tuiApplicationController struct {
	Query         string
	Offset        int
	Results       []*FileJob
	DocumentCount int
	Mutex         sync.Mutex
	DrawMutex     sync.Mutex
	SearchMutex   sync.Mutex

	// View requirements
	SpinString   string
	SpinLocation int
	SpinRun      int
}

func (cont *tuiApplicationController) SetQuery(q string) {
	cont.Mutex.Lock()
	cont.Query = q
	cont.Mutex.Unlock()
}

func (cont *tuiApplicationController) IncrementOffset() {
	cont.Offset++
}

func (cont *tuiApplicationController) DecrementOffset() {
	if cont.Offset > 0 {
		cont.Offset--
	}
}

func (cont *tuiApplicationController) ResetOffset() {
	cont.Offset = 0
}

func makeFmt(prefix string) string {
	return hex.EncodeToString(md5.New().Sum([]byte(fmt.Sprintf("%s%d", prefix, nowNanos()))))
}

func digitsCount(n int) int {
	res := 0
	for n > 0 {
		n /= 10
		res++
	}
	return res
}

// After any change is made that requires something drawn on the screen this is the method that does it
// NB this can never be called directly because it triggers a draw at the end.
func (cont *tuiApplicationController) drawView() {
	cont.DrawMutex.Lock()
	defer cont.DrawMutex.Unlock()

	// reset the elements by clearing out every one so we have a clean slate to start with
	for _, t := range tuiDisplayResults {
		t.Title.SetText("")
		t.Body.SetText("")
		t.SpacerOne.SetText("")
		t.SpacerTwo.SetText("")
		resultsFlex.ResizeItem(t.Body, 0, 0)
	}

	resultsCopy := make([]*FileJob, len(cont.Results))
	copy(resultsCopy, cont.Results)

	// rank all results
	// then go and get the relevant portion for display
	rankResults(cont.DocumentCount, resultsCopy)
	documentTermFrequency := calculateDocumentTermFrequency(iter.FromMany(resultsCopy...))

	// after ranking only get the details for as many as we actually need to
	// cut down on processing
	if len(resultsCopy) > len(tuiDisplayResults) {
		resultsCopy = resultsCopy[:len(tuiDisplayResults)]
	}

	// We use this to swap out the highlights after we escape to ensure that we don't escape
	// out own colours
	fmtBegin := makeFmt("begin_")
	fmtEnd := makeFmt("end_")

	// go and get the codeResults the user wants to see using selected as the offset to display from
	var codeResults []codeResult
	for i, res := range resultsCopy {
		if i < cont.Offset {
			continue
		}

		// TODO run in parallel for performance boost...
		snippet, ok := extractRelevantV3(res, documentTermFrequency, int(SnippetLength)).Head()
		if !ok { // false positive most likely
			continue
		}

		// now that we have the relevant portion we need to get just the bits related to highlight it correctly
		// which this method does. It takes in the snippet, we extract and all of the locations and then return
		l := getLocated(res.MatchLocations, snippet)
		coloredContent := str.HighlightString(snippet.Content, l, fmtBegin, fmtEnd)
		coloredContent = tview.Escape(coloredContent)

		coloredContent = strings.Replace(coloredContent, fmtBegin, "[red]", -1)
		coloredContent = strings.Replace(coloredContent, fmtEnd, "[white]", -1)

		lines := strings.Split(coloredContent, "\n")
		for i, line := range lines {
			lines[i] = fmt.Sprintf("[gray]%"+strconv.Itoa(digitsCount(snippet.LineEnd))+"d.[white] %s",
				snippet.LineStart+i,
				line,
			)
		}
		codeResults = append(codeResults, codeResult{
			Title:    res.Location,
			Content:  strings.Join(lines, "\n"),
			Score:    res.Score,
			Location: res.Location,
		})
	}

	// render out what the user wants to see based on the results that have been chosen
	for i, t := range codeResults {
		tuiDisplayResults[i].Title.SetText(fmt.Sprintf("[fuchsia]%s (%f)[-:-:-]", t.Title, t.Score))
		tuiDisplayResults[i].Body.SetText(t.Content)
		tuiDisplayResults[i].Location = t.Location

		// we need to update the item so that it displays everything we have put in
		resultsFlex.ResizeItem(tuiDisplayResults[i].Body, len(strings.Split(t.Content, "\n")), 0)
	}

	// because the search runs async through debounce we need to now draw
	tviewApplication.QueueUpdateDraw(func() {})
}

func (cont *tuiApplicationController) DoSearch() {
	cont.Mutex.Lock()
	query := cont.Query
	cont.Mutex.Unlock()

	cont.SearchMutex.Lock()
	defer cont.SearchMutex.Unlock()

	// have a spinner which indicates if things are running as we expect
	running := true
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			statusView.SetText(fmt.Sprintf("%s searching for '%s'", string(cont.SpinString[cont.SpinLocation]), query))
			tviewApplication.QueueUpdateDraw(func() {})
			cont.RotateSpin()
			time.Sleep(20 * time.Millisecond)
			if !running {
				wg.Done()
				return
			}
		}
	}()

	var results []*FileJob
	var status string

	if strings.TrimSpace(query) != "" {
		files := FindFiles()

		q, fuzzy := PreParseQuery(strings.Fields(query))

		fileReaderWorker := NewFileReaderWorker(fuzzy)
		toProcessQueue := fileReaderWorker.Start(files)
		fileSearcher := NewSearcherWorker(q)
		summaryQueue := fileSearcher.Start(toProcessQueue)

		// First step is to collect results so we can rank them
		for f := range summaryQueue {
			results = append(results, f)
		}

		plural := "s"
		if len(results) == 1 {
			plural = ""
		}

		status = fmt.Sprintf("%d result%s for '%s' from %d files", len(results), plural, query, fileReaderWorker.GetFileCount())
		cont.DocumentCount = fileReaderWorker.GetFileCount()
	}

	running = false
	wg.Wait()
	statusView.SetText(status)

	cont.Results = results
	cont.drawView()
}

func (cont *tuiApplicationController) RotateSpin() {
	cont.SpinRun++
	if cont.SpinRun == 4 {
		cont.SpinLocation++
		if cont.SpinLocation >= len(cont.SpinString) {
			cont.SpinLocation = 0
		}
		cont.SpinRun = 0
	}
}

// Sets up the UI components we need to actually display
var (
	overallFlex       *tview.Flex
	inputField        *tview.InputField
	queryFlex         *tview.Flex
	resultsFlex       *tview.Flex
	statusView        *tview.TextView
	tuiDisplayResults []displayResult
	tviewApplication  *tview.Application
	snippetInputField *tview.InputField
)

func NewTuiSearch() error {
	// start indexing by walking from the current directory and updating
	// this needs to run in the background with searches spawning from that
	tviewApplication = tview.NewApplication()
	applicationController := tuiApplicationController{
		Mutex:      sync.Mutex{},
		SpinString: `\|/-`,
	}

	// Create the elements we use to display the code results here
	for i := 1; i < 50; i++ {
		tuiDisplayResults = append(tuiDisplayResults, displayResult{
			Title: tview.NewTextView().
				SetDynamicColors(true).
				SetRegions(true).
				ScrollToBeginning(),
			Body: tview.NewTextView().
				SetDynamicColors(true).
				SetRegions(true).
				ScrollToBeginning(),
			BodyHeight: -1,
			SpacerOne:  tview.NewTextView(),
			SpacerTwo:  tview.NewTextView(),
		})
	}

	// input field which deals with the user input for the main search which ultimately triggers a search
	inputField = tview.NewInputField().
		SetFieldBackgroundColor(tcell.Color16).
		SetLabel("> ").
		SetLabelColor(tcell.ColorWhite).
		SetFieldWidth(0).
		SetDoneFunc(func(key tcell.Key) {
			// this deals with the keys that trigger "done" functions such as up/down/enter
			switch key {
			case tcell.KeyEnter:
				tviewApplication.Stop()
				// we want to work like fzf for piping into other things hence print out the selected version
				if len(applicationController.Results) != 0 {
					fmt.Println(tuiDisplayResults[0].Location)
				}
				os.Exit(0)
			case tcell.KeyTab:
				tviewApplication.SetFocus(snippetInputField)
			case tcell.KeyBacktab:
				tviewApplication.SetFocus(snippetInputField)
			case tcell.KeyUp:
				applicationController.DecrementOffset()
				// TODO: remove goroutines
				go applicationController.drawView()
			case tcell.KeyDown:
				applicationController.IncrementOffset()
				go applicationController.drawView()
			case tcell.KeyESC:
				tviewApplication.Stop()
				os.Exit(0)
			}
		}).
		SetChangedFunc(func(text string) {
			// after the text has changed set the query and trigger a search
			applicationController.ResetOffset() // reset so we are at the first element again
			applicationController.SetQuery(strings.TrimSpace(text))
			go applicationController.DoSearch()
		})

	// Decide how large a snippet we should be displaying
	snippetInputField = tview.NewInputField().
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetAcceptanceFunc(tview.InputFieldInteger).
		SetText(strconv.Itoa(int(SnippetLength))).
		SetFieldWidth(4).
		SetChangedFunc(func(text string) {
			SnippetLength = 300 // default
			if t, _ := strconv.Atoi(text); t != 0 {
				SnippetLength = int64(t)
			}
		}).
		SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyTab:
				tviewApplication.SetFocus(inputField)
			case tcell.KeyBacktab:
				tviewApplication.SetFocus(inputField)
			case tcell.KeyEnter:
				fallthrough
			case tcell.KeyUp:
				SnippetLength = min(SnippetLength+100, 8000)
				snippetInputField.SetText(fmt.Sprintf("%d", SnippetLength))
				go applicationController.DoSearch()
			case tcell.KeyPgUp:
				SnippetLength = min(SnippetLength+200, 8000)
				snippetInputField.SetText(fmt.Sprintf("%d", SnippetLength))
				go applicationController.DoSearch()
			case tcell.KeyDown:
				SnippetLength = max(100, SnippetLength-100)
				snippetInputField.SetText(fmt.Sprintf("%d", SnippetLength))
				go applicationController.DoSearch()
			case tcell.KeyPgDn:
				SnippetLength = max(100, SnippetLength-200)
				snippetInputField.SetText(fmt.Sprintf("%d", SnippetLength))
				go applicationController.DoSearch()
			case tcell.KeyESC:
				tviewApplication.Stop()
				os.Exit(0)
			}
		})

	statusView = tview.NewTextView().
		SetDynamicColors(false).
		SetRegions(false).
		ScrollToBeginning()

	// setup the flex containers to have everything rendered neatly
	queryFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(inputField, 0, 8, false).
		AddItem(snippetInputField, 5, 1, false)

	resultsFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	// add all of the display codeResults into the container ready to be populated
	for _, t := range tuiDisplayResults {
		resultsFlex.
			AddItem(t.SpacerOne, 1, 0, false).
			AddItem(t.Title, 1, 0, false).
			AddItem(t.SpacerTwo, 1, 0, false).
			AddItem(t.Body, t.BodyHeight, 1, false)
	}

	overallFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(queryFlex, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(statusView, 1, 0, false).
		AddItem(resultsFlex, 0, 1, false)

	return tviewApplication.
		SetRoot(overallFlex, true).
		SetFocus(inputField).
		Run()
}

func getLocated(matchLocations map[string]iter.Seq[[2]int], v3 Snippet) iter.Seq[[2]int] {
	// For all the match locations we have only keep the ones that should be inside
	// where we are matching
	return iter.Map(
		iter.Filter(
			iter.Flatten(
				iter.Values(
					iter.FromDict(
						matchLocations))),
			func(s [2]int) bool {
				return s[0] >= v3.Pos[0] && s[1] <= v3.Pos[1]
			}),
		func(s [2]int) [2]int {
			// Have to create a new one to avoid changing the position
			// unlike in others where we throw away the results afterwards
			return [2]int{
				s[0] - v3.Pos[0],
				s[1] - v3.Pos[0],
			}
		})
}
