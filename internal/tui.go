package internal

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"iter"
	"maps"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/rprtr258/cs/internal/core"
	"github.com/rprtr258/cs/internal/str"
)

func digitsCount(n int) int {
	res := 0
	for n > 0 {
		n /= 10
		res++
	}
	return res
}

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
	Results       []*core.FileJob
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
	cont.Offset = max(0, cont.Offset-1)
}

func (cont *tuiApplicationController) ResetOffset() {
	cont.Offset = 0
}

func (cont *tuiApplicationController) GetOffset() int {
	return cont.Offset
}

func getLocated(matchLocations map[string][][2]int, snippet core.Snippet) iter.Seq[[2]int] {
	// For all the match locations we have only keep the ones that should be inside
	// where we are matching
	locations := maps.Values(matchLocations)
	location := flatmap(locations, slices.Values)
	return filtermap(location, func(location [2]int) ([2]int, bool) {
		if location[0] < snippet.Pos[0] || location[1] > snippet.Pos[1] {
			return [2]int{}, false
		}

		// Have to create a new one to avoid changing the position
		// unlike in others where we throw away the results afterwards
		return [2]int{
			location[0] - snippet.Pos[0],
			location[1] - snippet.Pos[0],
		}, true
	})
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

	resultsCopy := make([]*core.FileJob, len(cont.Results))
	copy(resultsCopy, cont.Results)

	// rank all results
	// then go and get the relevant portion for display
	core.RankResults(cont.DocumentCount, resultsCopy)
	documentTermFrequency := core.CalculateDocumentTermFrequency(slices.Values(resultsCopy))

	// after ranking only get the details for as many as we actually need to
	// cut down on processing
	if len(resultsCopy) > len(tuiDisplayResults) {
		resultsCopy = resultsCopy[:len(tuiDisplayResults)]
	}

	// We use this to swap out the highlights after we escape to ensure that we don't escape
	// out own colours
	md5Digest := md5.New()
	fmtBegin := hex.EncodeToString(md5Digest.Sum([]byte(fmt.Sprintf("begin_%d", nowNanos()))))
	fmtEnd := hex.EncodeToString(md5Digest.Sum([]byte(fmt.Sprintf("end_%d", nowNanos()))))

	// go and get the codeResults the user wants to see using selected as the offset to display from
	codeResults := make([]codeResult, 0, len(resultsCopy))
	for i, res := range resultsCopy {
		if i < cont.Offset {
			continue
		}

		// TODO run in parallel for performance boost...
		snippet, ok := first(core.ExtractRelevantV3(res, documentTermFrequency, core.SnippetLength))
		if !ok { // false positive most likely
			continue
		}

		// now that we have the relevant portion we need to get just the bits related to highlight it correctly
		// which this method does. It takes in the snippet, we extract and all of the locations and then return
		l := getLocated(res.MatchLocations, snippet)
		coloredContent := str.HighlightString(snippet.Content, l, fmtBegin, fmtEnd)
		coloredContent = tview.Escape(coloredContent)

		coloredContent = strings.ReplaceAll(coloredContent, fmtBegin, "[red]")
		coloredContent = strings.ReplaceAll(coloredContent, fmtEnd, "[white]")

		maxLineNumberLen := digitsCount(snippet.LinePos[1])

		lines := strings.Split(coloredContent, "\n")
		for i, line := range lines {
			lines[i] = fmt.Sprintf(
				"[gray]%"+strconv.Itoa(maxLineNumberLen)+"d.[white] %s",
				snippet.LinePos[0]+i,
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

	var results []*core.FileJob
	var status string
	if strings.TrimSpace(query) != "" {
		files := core.FindFiles(query)
		toProcessCh := make(chan *core.FileJob, runtime.NumCPU()) // Files to be read into memory for processing
		summaryCh := make(chan *core.FileJob, runtime.NumCPU())   // Files that match and need to be displayed

		q, fuzzy := core.PreParseQuery(strings.Fields(query))
		fileReaderWorker := core.NewFileReaderWorker(files, toProcessCh, fuzzy)

		go fileReaderWorker.Start()
		go core.NewSearcherWorker(toProcessCh, summaryCh, q)

		// First step is to collect results so we can rank them
		fileMatches := []string{}
		for f := range summaryCh {
			results = append(results, f)
			fileMatches = append(fileMatches, f.Location)
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
		cont.SpinLocation = (cont.SpinLocation + 1) % len(cont.SpinString)
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

// setup debounce to improve ui feel
var debounced = NewDebouncer(200 * time.Millisecond)

func NewTuiSearch() error {
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

	// start indexing by walking from the current directory and updating
	// this needs to run in the background with searches spawning from that
	tviewApplication = tview.NewApplication()
	applicationController := tuiApplicationController{
		Mutex:      sync.Mutex{},
		SpinString: `\|/-`,
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
				os.Exit(0) // TODO: remove
			case tcell.KeyTab, tcell.KeyBacktab:
				tviewApplication.SetFocus(snippetInputField)
			case tcell.KeyUp:
				applicationController.DecrementOffset()
				debounced(applicationController.drawView)
			case tcell.KeyDown:
				applicationController.IncrementOffset()
				debounced(applicationController.drawView)
			case tcell.KeyESC:
				tviewApplication.Stop()
				os.Exit(0) // TODO: remove
			}
		}).
		SetChangedFunc(func(text string) {
			// after the text has changed set the query and trigger a search
			applicationController.ResetOffset() // reset so we are at the first element again
			applicationController.SetQuery(strings.TrimSpace(text))
			debounced(applicationController.DoSearch)
		})

	// Decide how large a snippet we should be displaying
	snippetInputField = tview.NewInputField().
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetAcceptanceFunc(tview.InputFieldInteger).
		SetText(strconv.Itoa(core.SnippetLength)).
		SetFieldWidth(4).
		SetChangedFunc(func(text string) {
			if strings.TrimSpace(text) == "" {
				core.SnippetLength = 300 // default
			} else {
				core.SnippetLength = tryParseInt(text, 300)
			}
		}).
		SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyTab:
				tviewApplication.SetFocus(inputField)
			case tcell.KeyBacktab:
				tviewApplication.SetFocus(inputField)
			case tcell.KeyEnter, tcell.KeyUp:
				core.SnippetLength = min(core.SnippetLength+100, 8000)
				snippetInputField.SetText(fmt.Sprintf("%d", core.SnippetLength))
				debounced(applicationController.DoSearch)
			case tcell.KeyPgUp:
				core.SnippetLength = min(core.SnippetLength+200, 8000)
				snippetInputField.SetText(fmt.Sprintf("%d", core.SnippetLength))
				debounced(applicationController.DoSearch)
			case tcell.KeyDown:
				core.SnippetLength = max(100, core.SnippetLength-100)
				snippetInputField.SetText(fmt.Sprintf("%d", core.SnippetLength))
				debounced(applicationController.DoSearch)
			case tcell.KeyPgDn:
				core.SnippetLength = max(100, core.SnippetLength-200)
				snippetInputField.SetText(fmt.Sprintf("%d", core.SnippetLength))
				debounced(applicationController.DoSearch)
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
