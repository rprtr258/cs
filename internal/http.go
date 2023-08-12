package internal

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/boyter/gocodewalker"
	"github.com/rprtr258/fun"
	"github.com/rprtr258/fun/iter"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"github.com/boyter/cs/str"
)

func StartHttpServer(addr, SearchTemplate, DisplayTemplate string) error {
	http.HandleFunc("/file/raw/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Replace(r.URL.Path, "/file/raw/", "", 1)

		log.Info().
			Caller().
			Str("path", path).
			Msg("raw page")

		w.Header().Set("Content-Type", "text/plain")
		http.ServeFile(w, r, path)
	})

	http.HandleFunc("/file/", func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		startPos := parseIntOrDefault(r.URL.Query().Get("sp"), 0)
		endPos := parseIntOrDefault(r.URL.Query().Get("ep"), 0)

		path := strings.TrimPrefix(r.URL.Path, "/file/")

		log.Info().
			Caller().
			Int("startpos", startPos).
			Int("endpos", endPos).
			Str("path", path).
			Msg("file view page")

		content, err := os.ReadFile(fun.If(
			strings.TrimSpace(Directory) != "",
			"/"+path,
			path,
		))
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
		fmtBegin := makeFmt("begin_")
		fmtEnd := makeFmt("end_")

		coloredContent := fun.Pipe(string(content),
			func(s string) string {
				return str.HighlightString(s, iter.FromMany([2]int{startPos, endPos}), fmtBegin, fmtEnd)
			},
			html.EscapeString,
			func(s string) string {
				return strings.Replace(s, fmtBegin, fmt.Sprintf(`<strong id="%d">`, startPos), -1)
			},
			func(s string) string {
				return strings.Replace(s, fmtEnd, "</strong>", -1)
			},
		)

		t := _httpFileTemplate
		if DisplayTemplate != "" {
			t = template.Must(template.
				New("display.tmpl").
				ParseFiles(DisplayTemplate))
		}

		if err = t.Execute(w, fileDisplay{
			Location:            path,
			Content:             template.HTML(coloredContent),
			RuntimeMilliseconds: time.Since(startTime),
		}); err != nil {
			panic(err)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		startTime := nowMillis()
		query := r.URL.Query().Get("q")
		snippetLength := parseIntOrDefault(r.URL.Query().Get("ss"), 300)
		ext := r.URL.Query().Get("ext")
		page := parseIntOrDefault(r.URL.Query().Get("p"), 0)
		const pageSize = 20

		log.Info().
			Caller().
			Msg("search page")

		var results []*FileJob
		var fileCount int
		if query != "" {
			log.Info().
				Caller().
				Str("query", query).
				Int("snippetlength", snippetLength).
				Str("ext", ext).
				Msg("search query")

			// If the user asks we should look back till we find the .git or .hg directory and start the search from there

			DirFilePaths = fun.If(
				strings.TrimSpace(Directory) == "",
				".",
				Directory,
			)
			if FindRoot {
				DirFilePaths = gocodewalker.FindRepositoryRoot(DirFilePaths)
			}

			AllowListExtensions = fun.If(
				len(ext) == 0,
				cli.NewStringSlice(),
				cli.NewStringSlice(ext),
			)

			// walk back through the query to see if we have a shorter one that matches
			files := FindFiles()

			q, fuzzy := PreParseQuery(strings.Fields(query))

			fileReaderWorker := NewFileReaderWorker(fuzzy)
			toProcessQueue := fileReaderWorker.Start(files)

			summaryQueue := NewSearcherWorker(q).Start(toProcessQueue)
			for f := range summaryQueue {
				results = append(results, f)
			}

			fileCount = fileReaderWorker.GetFileCount()
			rankResults(fileCount, results)
		}

		// Create a random str to define where the start and end of
		// out highlight should be which we swap out later after we have
		// HTML escaped everything
		md5Digest := md5.New()
		fmtBegin := hex.EncodeToString(md5Digest.Sum([]byte(fmt.Sprintf("begin_%d", nowNanos()))))
		fmtEnd := hex.EncodeToString(md5Digest.Sum([]byte(fmt.Sprintf("end_%d", nowNanos()))))

		// if we have more than the page size of results, lets just show the first page
		pages := calculatePages(results, pageSize, query, snippetLength, ext).ToSlice()

		displayResults := results[:min(pageSize, len(results))]
		if page != 0 && page <= len(pages) {
			displayResults = results[page*pageSize : min((page+1)*pageSize, len(results))]
		}

		// loop over all results so we can get the facets
		extensionFacets := iter.ToCounterBy(
			iter.FromMany(results...),
			func(res *FileJob) string {
				return gocodewalker.GetExtension(res.Filename)
			})

		documentTermFrequency := calculateDocumentTermFrequency(iter.FromMany(results...))

		searchResults := make([]searchResult, len(displayResults))
		for i, res := range displayResults {
			v3, _ := extractRelevantV3(res, documentTermFrequency, snippetLength).Head()
			searchResults[i] = searchResult{
				Title:    res.Location,
				Location: res.Location,
				// We want to escape the output, so we highlight, then escape then replace
				// our special start and end strings with actual HTML
				Content: []template.HTML{template.HTML(fun.If(
					v3.Pos[1] == 0,
					// If endpos = 0 don't highlight anything because it means its a filename match
					v3.Content,
					// We have the snippet so now we need to highlight it
					// we get all the locations that fall in the snippet length
					// and then remove the length of the snippet cut which
					// makes out location line up with the snippet size
					fun.Pipe(v3.Content,
						func(s string) string {
							return str.HighlightString(s, iter.
								Flatten(iter.Values(iter.FromDict(res.MatchLocations))).
								Filter(func(s [2]int) bool {
									return s[0] >= v3.Pos[0] && s[1] <= v3.Pos[1]
								}).
								Map(func(s [2]int) [2]int {
									return [2]int{
										s[0] - v3.Pos[0],
										s[1] - v3.Pos[0],
									}
								}), fmtBegin, fmtEnd)
						},
						html.EscapeString,
						func(s string) string { return strings.Replace(s, fmtBegin, "<strong>", -1) },
						func(s string) string { return strings.Replace(s, fmtEnd, "</strong>", -1) },
					),
				))},
				Pos:   v3.Pos,
				Score: res.Score,
			}
		}

		t := _httpSearchTemplate
		if SearchTemplate != "" {
			// If we have been supplied a template then load it up
			t = template.Must(template.New("search.tmpl").ParseFiles(SearchTemplate))
		}

		if err := t.Execute(w, search{
			SearchTerm:          query,
			SnippetSize:         snippetLength,
			Results:             searchResults,
			ResultsCount:        len(results),
			RuntimeMilliseconds: nowMillis() - startTime,
			ProcessedFileCount:  fileCount,
			ExtensionFacet:      calculateExtensionFacet(extensionFacets, query, snippetLength),
			Pages:               pages,
			Ext:                 ext,
		}); err != nil {
			panic(err)
		}
	})

	log.Info().
		Caller().
		Str("address", addr).
		Msg("ready to serve requests")
	return http.ListenAndServe(addr, nil)
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

	slices.SortFunc(ef, func(a, b facetResult) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		// If the same count sort by the name to ensure it's consistent on the display
		return strings.Compare(a.Title, b.Title)
	})

	return ef
}

func calculatePages(results []*FileJob, pageSize int, query string, snippetLength int, ext string) iter.Seq[pageResult] {
	return iter.Map(
		iter.FromRange(0, (len(results)+pageSize-1)/pageSize, 1),
		func(i int) pageResult {
			return pageResult{
				SearchTerm:  query,
				SnippetSize: snippetLength,
				Value:       i,
				Name:        strconv.Itoa(i + 1),
				Ext:         ext,
			}
		})
}

func parseIntOrDefault(x string, def int) int {
	t, err := strconv.Atoi(x)
	if err != nil {
		return def
	}

	return t
}
