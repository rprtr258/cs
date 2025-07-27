package internal

import (
	"cmp"
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/boyter/gocodewalker"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"github.com/rprtr258/cs/internal/core"
	"github.com/rprtr258/cs/internal/str"
	. "github.com/rprtr258/cs/internal/utils"
)

type searchResult struct {
	Title    string
	Location string
	Content  []template.HTML
	Pos      [2]int
	Score    float64
}

type facetResult struct {
	Title       string
	Count       int
	SearchTerm  string
	SnippetSize int
}

type pageResult struct {
	SearchTerm  string
	SnippetSize int
	Value       int
	Name        string
	Ext         string
}

type search struct {
	SearchTerm          string
	SnippetSize         int
	Results             []searchResult
	ResultsCount        int
	RuntimeMilliseconds int64
	ProcessedFileCount  int
	ExtensionFacet      []facetResult
	Pages               []pageResult
	Ext                 string
}

var (
	//go:embed templates/display.tmpl
	_displayTmpl         string
	_templateHTMLDisplay = template.Must(template.New("display.tmpl").Parse(_displayTmpl))

	//go:embed templates/search.tmpl
	_searchTmpl         string
	_templateHTMLSearch = template.Must(template.New("search.tmpl").Parse(_searchTmpl))
)

func tryParseInt(x string, def int) int {
	t, err := strconv.Atoi(x)
	if err != nil {
		return def
	}

	return t
}

func StartHttpServer() error {
	http.HandleFunc("/file/raw/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Replace(r.URL.Path, "/file/raw/", "", 1)

		log.Info().
			Caller().
			Str("path", path).
			Msg("raw page")

		w.Header().Set("Content-Type", "text/plain")
		http.ServeFile(w, r, path)
	})

	templateDisplay := _templateHTMLDisplay
	if core.DisplayTemplate != "" {
		templateDisplay = template.Must(template.New("display.tmpl").ParseFiles(core.DisplayTemplate))
	}

	http.HandleFunc("/file/", func(w http.ResponseWriter, r *http.Request) {
		startTime := NowMillis()
		startPos := tryParseInt(r.URL.Query().Get("sp"), 0)
		endPos := tryParseInt(r.URL.Query().Get("ep"), 0)

		path := strings.Replace(r.URL.Path, "/file/", "", 1)

		log.Info().
			Caller().
			Int("startpos", startPos).
			Int("endpos", endPos).
			Str("path", path).
			Msg("file view page")

		if strings.TrimSpace(core.Directory) != "" {
			path = "/" + path
		}

		content, err := os.ReadFile(path)
		if err != nil {
			log.Error().
				Caller().
				Int("startpos", startPos).
				Int("endpos", endPos).
				Str("path", path).
				Msg("error reading file")
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		}

		fmtBegin, fmtEnd := CreateFmts()

		coloredContent := str.HighlightString(string(content), slices.Values([][2]int{{startPos, endPos}}), fmtBegin, fmtEnd)
		coloredContent = html.EscapeString(coloredContent)
		coloredContent = strings.NewReplacer(
			fmtBegin, fmt.Sprintf(`<strong id="%d">`, startPos),
			fmtEnd, "</strong>",
		).Replace(coloredContent)

		err = templateDisplay.Execute(w, struct {
			Location            string
			Content             template.HTML
			RuntimeMilliseconds int64
		}{
			Location:            path,
			Content:             template.HTML(coloredContent),
			RuntimeMilliseconds: NowMillis() - startTime,
		})
		if err != nil {
			panic(err)
		}
	})

	templateSearch := _templateHTMLSearch
	if core.SearchTemplate != "" {
		// If we have been supplied a template then load it up
		templateSearch = template.Must(template.New("search.tmpl").ParseFiles(core.SearchTemplate))
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		startTime := NowMillis()
		query := r.URL.Query().Get("q")
		snippetLength := tryParseInt(r.URL.Query().Get("ss"), 300)
		ext := r.URL.Query().Get("ext")
		page := tryParseInt(r.URL.Query().Get("p"), 0)
		pageSize := 20

		log.Info().
			Caller().
			Msg("search page")

		var results []*core.FileJob
		var fileCount int
		if query != "" {
			log.Info().
				Caller().
				Str("query", query).
				Int("snippetlength", snippetLength).
				Str("ext", ext).
				Msg("search query")

			// If the user asks we should look back till we find the .git or .hg directory and start the search from there

			core.DirFilePaths = []string{"."}
			if strings.TrimSpace(core.Directory) != "" {
				core.DirFilePaths = []string{core.Directory}
			}
			if core.FindRoot {
				core.DirFilePaths[0] = gocodewalker.FindRepositoryRoot(core.DirFilePaths[0])
			}

			core.AllowListExtensions = cli.NewStringSlice()
			if ext != "" {
				core.AllowListExtensions = cli.NewStringSlice(ext)
			}

			// walk back through the query to see if we have a shorter one that matches
			files := core.FindFiles(query)

			toProcessCh := make(chan *core.FileJob, runtime.NumCPU()) // Files to be read into memory for processing
			summaryCh := make(chan *core.FileJob, runtime.NumCPU())   // Files that match and need to be displayed

			q, fuzzy := core.PreParseQuery(strings.Fields(query))

			fileReaderWorker := core.NewFileReaderWorker(files, toProcessCh, fuzzy)

			go fileReaderWorker.Start()
			go core.NewSearcherWorker(toProcessCh, summaryCh, q)

			for f := range summaryCh {
				results = append(results, f)
			}

			fileCount = fileReaderWorker.GetFileCount()
			core.RankResults(fileReaderWorker.GetFileCount(), results)
		}

		fmtBegin, fmtEnd := CreateFmts()

		documentTermFrequency := core.CalculateDocumentTermFrequency(slices.Values(results))

		// if we have more than the page size of results, lets just show the first page
		pages := calculatePages(results, pageSize, query, snippetLength, ext)

		displayResults := results
		if displayResults != nil && len(displayResults) > pageSize {
			displayResults = displayResults[:pageSize]
		}
		if page != 0 && page <= len(pages) {
			displayResults = results[page*pageSize : min(len(results), page*pageSize+pageSize)]
		}

		// loop over all results so we can get the facets
		extensionFacets := map[string]int{}
		for _, res := range results {
			extensionFacets[gocodewalker.GetExtension(res.Filename)] += 1
		}

		searchResults := make([]searchResult, 0, len(displayResults))
		for _, res := range displayResults {
			v3, _ := first(core.ExtractRelevantV3(res, documentTermFrequency, snippetLength))

			// We have the snippet so now we need to highlight it
			// we get all the locations that fall in the snippet length
			// and then remove the length of the snippet cut which
			// makes out location line up with the snippet size
			l := func(yield func([2]int) bool) {
				for _, value := range res.MatchLocations {
					for _, s := range value {
						if s[0] >= v3.Pos[0] && s[1] <= v3.Pos[1] {
							if !yield([2]int{
								s[0] - v3.Pos[0],
								s[1] - v3.Pos[0],
							}) {
								return
							}
						}
					}
				}
			}

			// We want to escape the output, so we highlight, then escape then replace
			// our special start and end strings with actual HTML
			coloredContent := v3.Content
			// If endpos = 0 don't highlight anything because it means its a filename match
			if v3.Pos[1] != 0 {
				coloredContent = str.HighlightString(v3.Content, l, fmtBegin, fmtEnd)
				coloredContent = html.EscapeString(coloredContent)
				coloredContent = strings.NewReplacer(
					fmtBegin, "<strong>",
					fmtEnd, "</strong>",
				).Replace(coloredContent)
			}

			searchResults = append(searchResults, searchResult{
				Title:    res.Location,
				Location: res.Location,
				Content:  []template.HTML{template.HTML(coloredContent)},
				Pos:      v3.Pos,
				Score:    res.Score,
			})
		}

		err := templateSearch.Execute(w, search{
			SearchTerm:          query,
			SnippetSize:         snippetLength,
			Results:             searchResults,
			ResultsCount:        len(results),
			RuntimeMilliseconds: NowMillis() - startTime,
			ProcessedFileCount:  fileCount,
			ExtensionFacet:      calculateExtensionFacet(extensionFacets, query, snippetLength),
			Pages:               pages,
			Ext:                 ext,
		})
		if err != nil {
			panic(err)
		}
	})

	log.Info().
		Caller().
		Str("address", core.Address).
		Msg("ready to serve requests")
	return http.ListenAndServe(core.Address, nil)
}

func calculateExtensionFacet(extensionFacets map[string]int, query string, snippetLength int) []facetResult {
	ef := make([]facetResult, 0, len(extensionFacets))
	for k, v := range extensionFacets {
		ef = append(ef, facetResult{
			Title:       k,
			Count:       v,
			SearchTerm:  query,
			SnippetSize: snippetLength,
		})
	}

	slices.SortFunc(ef, func(i, j facetResult) int {
		// If the same count sort by the name to ensure it's consistent on the display
		return cmp.Or(
			cmp.Compare(i.Count, j.Count),
			strings.Compare(i.Title, j.Title),
		)
	})

	return ef
}

// TODO: simplify to only third case
func calculatePages(results []*core.FileJob, pageSize int, query string, snippetLength int, ext string) []pageResult {
	if len(results) == 0 {
		return nil
	}

	if len(results) <= pageSize {
		return []pageResult{{
			SearchTerm:  query,
			SnippetSize: snippetLength,
			Value:       0,
			Name:        "1",
		}}
	}

	pages := make([]pageResult, (len(results)+pageSize-1)/pageSize)
	for i := range pages {
		pages[i] = pageResult{
			SearchTerm:  query,
			SnippetSize: snippetLength,
			Value:       i,
			Name:        strconv.Itoa(i + 1),
			Ext:         ext,
		}
	}
	return pages
}
