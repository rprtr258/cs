package main

import (
	"log"
	"os"
	"runtime/pprof"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/rprtr258/cs/internal"
	"github.com/rprtr258/cs/internal/core"
)

const _version = "1.4.0"

// _httpServer indicates if we should fork into HTTP mode or not
var _httpServer = false

var _app = cli.App{
	Name:    "cs",
	Version: _version,
	Authors: []*cli.Author{
		{
			Name:  "Ben Boyter",
			Email: "ben@boyter.org",
		},
	},
	Usage: `code spelunker (cs) code search.

cs recursively searches the current directory using some boolean logic
optionally combined with regular expressions.

Works via command line where passed in arguments are the search terms
or in a TUI mode with no arguments. Can also run in HTTP mode with
the -d or --http-server flag.

Searches by default use AND boolean syntax for all terms
 - exact match using quotes "find this"
 - fuzzy match within 1 or 2 distance fuzzy~1 fuzzy~2
 - negate using NOT such as pride NOT prejudice
 - regex with toothpick syntax /pr[e-i]de/

Searches can fuzzy match which files are searched by adding
the following syntax

 - test file:test
 - stuff filename:.go

Files that are searched will be limited to those that fuzzy
match test for the first example and .go for the second.
Example search that uses all current functionality
 - darcy NOT collins wickham~1 "ten thousand a year" /pr[e-i]de/ file:test

The default input field in tui mode supports some nano commands
- CTRL+a move to the beginning of the input
- CTRL+e move to the end of the input
- CTRL+k to clear from the cursor location forward
`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "address",
			Destination: &core.Address,
			Value:       ":8080",
			Usage:       "address and port to listen to in HTTP mode",
		},
		&cli.BoolFlag{
			Name:        "http-server",
			Destination: &_httpServer,
			Aliases:     []string{"d"},
			Usage:       "start http server for search",
		},
		&cli.BoolFlag{
			Destination: &core.IncludeBinaryFiles,
			Name:        "binary",
			Usage:       "set to disable binary file detection and search binary files",
		},
		&cli.BoolFlag{
			Destination: &core.IgnoreIgnoreFile,
			Name:        "no-ignore",
			Usage:       "disables .ignore file logic",
		},
		&cli.BoolFlag{
			Destination: &core.IgnoreGitIgnore,
			Name:        "no-gitignore",
			Usage:       "disables .gitignore file logic",
		},
		&cli.IntFlag{
			Destination: &core.SnippetLength,
			Name:        "snippet-length",
			Aliases:     []string{"n"},
			Value:       300,
			Usage:       "size of the snippet to display",
		},
		&cli.IntFlag{
			Destination: &core.SnippetCount,
			Name:        "snippet-count",
			Aliases:     []string{"s"},
			Value:       1,
			Usage:       "number of snippets to display",
		},
		&cli.BoolFlag{
			Destination: &core.IncludeHidden,
			Name:        "hidden",
			Usage:       "include hidden files",
		},
		&cli.StringSliceFlag{
			Destination: core.AllowListExtensions,
			Name:        "include-ext",
			Aliases:     []string{"i"},
			Usage:       "limit to file extensions (N.B. case sensitive) [comma separated list: e.g. go,java,js,C,cpp]",
		},
		&cli.BoolFlag{
			Destination: &core.FindRoot,
			Name:        "find-root",
			Aliases:     []string{"r"},
			Usage:       "attempts to find the root of this repository by traversing in reverse looking for .git or .hg",
		},
		&cli.StringSliceFlag{
			Destination: &core.PathDenylist,
			Name:        "exclude-dir",
			Value:       cli.NewStringSlice(".git", ".hg", ".svn", ".jj"),
			Usage:       "directories to exclude",
		},
		&cli.BoolFlag{
			Destination: &core.CaseSensitive,
			Name:        "case-sensitive",
			Aliases:     []string{"c"},
			Usage:       "make the search case sensitive",
		},
		&cli.StringFlag{
			Destination: &core.SearchTemplate,
			Name:        "template-search",
			Usage:       "path to search template for custom styling",
		},
		&cli.StringFlag{
			Destination: &core.DisplayTemplate,
			Name:        "template-display",
			Usage:       "path to display template for custom styling",
		},
		&cli.StringSliceFlag{
			Destination: &core.LocationExcludePattern,
			Name:        "exclude-pattern",
			Aliases:     []string{"x"},
			Usage:       "file and directory locations matching case sensitive patterns will be ignored [comma separated list: e.g. vendor,_test.go]",
		},
		&cli.BoolFlag{
			Destination: &core.IncludeMinified,
			Name:        "min",
			Usage:       "include minified files",
		},
		&cli.IntFlag{
			Destination: &core.MinifiedLineByteLength,
			Name:        "min-line-length",
			Value:       255,
			Usage:       "number of bytes per average line for file to be considered minified",
		},
		&cli.IntFlag{
			Destination: &core.MaxReadSizeBytes,
			Name:        "max-read-size-bytes",
			Value:       1_000_000,
			Usage:       "number of bytes to read into a file with the remaining content ignored",
		},
		&cli.StringFlag{
			Destination: &core.Format,
			Name:        "format",
			Aliases:     []string{"f"},
			Value:       "text",
			Usage:       "set output format [text, json, vimgrep]",
			Action: func(_ *cli.Context, s string) error {
				switch s {
				case "text", "json", "vimgrep":
					return nil
				default:
					return cli.Exit("invalid format", 1)
				}
			},
		},
		&cli.StringFlag{
			Destination: &core.Ranker,
			Name:        "ranker",
			Value:       "bm25",
			Usage:       "set ranking algorithm [simple, tfidf, tfidf2, bm25]",
			Action: func(_ *cli.Context, s string) error {
				switch s {
				case "simple", "tfidf", "tfidf2", "bm25":
					return nil
				default:
					return cli.Exit("invalid ranker", 1)
				}
			},
		},
		&cli.StringFlag{
			Destination: &core.FileOutput,
			Name:        "output",
			Aliases:     []string{"o"},
			Usage:       "output filename (default stdout)",
		},
		&cli.StringFlag{
			Destination: &core.Directory,
			Name:        "dir",
			Usage:       "directory to search, if not set defaults to current working directory",
		},
	},
	Action: func(ctx *cli.Context) error {
		core.SearchString = ctx.Args().Slice()

		core.DirFilePaths = []string{"."}
		if strings.TrimSpace(core.Directory) != "" {
			core.DirFilePaths = []string{core.Directory}
		}

		// If there are arguments we want to print straight out to the console
		// otherwise we should enter interactive tui mode
		switch {
		case _httpServer:
			return internal.StartHttpServer()
		case len(core.SearchString) != 0:
			internal.NewConsoleSearch()
			return nil
		default:
			return internal.NewTuiSearch()
		}
	},
}

func main() {
	if core.Pprof {
		f, _ := os.Create("profile.pprof")
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if err := _app.Run(os.Args); err != nil {
		log.Fatalln(err.Error())
	}
}
