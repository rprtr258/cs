package internal

import (
	"cmp"
	"crypto/md5"
	"encoding/hex"
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

	"github.com/rprtr258/cs/str"
)

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
	if DisplayTemplate != "" {
		templateDisplay = template.Must(template.New("display.tmpl").ParseFiles(DisplayTemplate))
	}

	http.HandleFunc("/file/", func(w http.ResponseWriter, r *http.Request) {
		startTime := nowMillis()
		startPos := tryParseInt(r.URL.Query().Get("sp"), 0)
		endPos := tryParseInt(r.URL.Query().Get("ep"), 0)

		path := strings.Replace(r.URL.Path, "/file/", "", 1)

		log.Info().
			Caller().
			Int("startpos", startPos).
			Int("endpos", endPos).
			Str("path", path).
			Msg("file view page")

		if strings.TrimSpace(Directory) != "" {
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

		// Create a random str to define where the start and end of
		// out highlight should be which we swap out later after we have
		// HTML escaped everything
		md5Digest := md5.New()
		fmtBegin := hex.EncodeToString(md5Digest.Sum([]byte(fmt.Sprintf("begin_%d", nowNanos()))))
		fmtEnd := hex.EncodeToString(md5Digest.Sum([]byte(fmt.Sprintf("end_%d", nowNanos()))))

		coloredContent := str.HighlightString(string(content), [][2]int{{startPos, endPos}}, fmtBegin, fmtEnd)

		coloredContent = html.EscapeString(coloredContent)
		coloredContent = strings.ReplaceAll(coloredContent, fmtBegin, fmt.Sprintf(`<strong id="%d">`, startPos))
		coloredContent = strings.ReplaceAll(coloredContent, fmtEnd, "</strong>")

		err = templateDisplay.Execute(w, fileDisplay{
			Location:            path,
			Content:             template.HTML(coloredContent),
			RuntimeMilliseconds: nowMillis() - startTime,
		})
		if err != nil {
			panic(err)
		}
	})

	templateSearch := _templateHTMLSearch
	if SearchTemplate != "" {
		// If we have been supplied a template then load it up
		templateSearch = template.Must(template.New("search.tmpl").ParseFiles(SearchTemplate))
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		startTime := nowMillis()
		query := r.URL.Query().Get("q")
		snippetLength := tryParseInt(r.URL.Query().Get("ss"), 300)
		ext := r.URL.Query().Get("ext")
		page := tryParseInt(r.URL.Query().Get("p"), 0)
		pageSize := 20

		var results []*FileJob
		var fileCount int

		log.Info().
			Caller().
			Msg("search page")

		if query != "" {
			log.Info().
				Caller().
				Str("query", query).
				Int("snippetlength", snippetLength).
				Str("ext", ext).
				Msg("search query")

			// If the user asks we should look back till we find the .git or .hg directory and start the search from there

			DirFilePaths = []string{"."}
			if strings.TrimSpace(Directory) != "" {
				DirFilePaths = []string{Directory}
			}
			if FindRoot {
				DirFilePaths[0] = gocodewalker.FindRepositoryRoot(DirFilePaths[0])
			}

			AllowListExtensions = cli.NewStringSlice()
			if ext != "" {
				AllowListExtensions = cli.NewStringSlice(ext)
			}

			// walk back through the query to see if we have a shorter one that matches
			files := FindFiles(query)

			toProcessQueue := make(chan *FileJob, runtime.NumCPU()) // Files to be read into memory for processing
			summaryQueue := make(chan *FileJob, runtime.NumCPU())   // Files that match and need to be displayed

			q, fuzzy := PreParseQuery(strings.Fields(query))

			fileReaderWorker := NewFileReaderWorker(files, toProcessQueue, fuzzy)
			fileSearcher := NewSearcherWorker(toProcessQueue, summaryQueue, q)

			go fileReaderWorker.Start()
			go fileSearcher.Start()

			for f := range summaryQueue {
				results = append(results, f)
			}

			fileCount = fileReaderWorker.GetFileCount()
			rankResults(fileReaderWorker.GetFileCount(), results)
		}

		// Create a random str to define where the start and end of
		// out highlight should be which we swap out later after we have
		// HTML escaped everything
		md5Digest := md5.New()
		fmtBegin := hex.EncodeToString(md5Digest.Sum([]byte(fmt.Sprintf("begin_%d", nowNanos()))))
		fmtEnd := hex.EncodeToString(md5Digest.Sum([]byte(fmt.Sprintf("end_%d", nowNanos()))))

		documentTermFrequency := calculateDocumentTermFrequency(results)

		var searchResults []searchResult
		extensionFacets := map[string]int{}

		// if we have more than the page size of results, lets just show the first page
		displayResults := results
		pages := calculatePages(results, pageSize, query, snippetLength, ext)

		if displayResults != nil && len(displayResults) > pageSize {
			displayResults = displayResults[:pageSize]
		}
		if page != 0 && page <= len(pages) {
			end := page*pageSize + pageSize
			if end > len(results) {
				end = len(results)
			}

			displayResults = results[page*pageSize : end]
		}

		// loop over all results so we can get the facets
		for _, res := range results {
			extensionFacets[gocodewalker.GetExtension(res.Filename)] = extensionFacets[gocodewalker.GetExtension(res.Filename)] + 1
		}

		for _, res := range displayResults {
			v3 := extractRelevantV3(res, documentTermFrequency, snippetLength)[0]

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

			// We want to escape the output, so we highlight, then escape then replace
			// our special start and end strings with actual HTML
			coloredContent := v3.Content
			// If endpos = 0 don't highlight anything because it means its a filename match
			if v3.Pos[1] != 0 {
				coloredContent = str.HighlightString(v3.Content, l, fmtBegin, fmtEnd)
				coloredContent = html.EscapeString(coloredContent)
				coloredContent = strings.ReplaceAll(coloredContent, fmtBegin, "<strong>")
				coloredContent = strings.ReplaceAll(coloredContent, fmtEnd, "</strong>")
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
			RuntimeMilliseconds: nowMillis() - startTime,
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
		Str("address", Address).
		Msg("ready to serve requests")
	return http.ListenAndServe(Address, nil)
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
		if i.Count != j.Count {
			return cmp.Compare(i.Count, j.Count)
		}
		return strings.Compare(i.Title, j.Title)
	})

	return ef
}

// TODO: simplify to only third case
func calculatePages(results []*FileJob, pageSize int, query string, snippetLength int, ext string) []pageResult {
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

func tryParseInt(x string, def int) int {
	t, err := strconv.Atoi(x)
	if err != nil {
		return def
	}

	return t
}
