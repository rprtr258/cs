package main

import (
	"os"
	"strings"

	"github.com/rprtr258/fun"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"github.com/boyter/cs/internal"
)

const _version = "1.3.0"

var app = cli.App{
	Name: "cs",
	Usage: `code spelunker (cs) code search.
Version ` + _version + `
Ben Boyter <ben@boyter.org>

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
	Version: _version,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Destination: &internal.IncludeBinaryFiles,
			Name:        "binary",
			Usage:       "set to disable binary file detection and search binary files",
		},
		&cli.BoolFlag{
			Destination: &internal.IgnoreIgnoreFile,
			Name:        "no-ignore",
			Usage:       "disables .ignore file logic",
		},
		&cli.BoolFlag{
			Destination: &internal.IgnoreGitIgnore,
			Name:        "no-gitignore",
			Usage:       "disables .gitignore file logic",
		},
		&cli.Int64Flag{
			Destination: &internal.SnippetLength,
			Name:        "snippet-length",
			Aliases:     []string{"n"},
			Value:       300,
			Usage:       "size of the snippet to display",
		},
		&cli.IntFlag{
			Destination: &internal.SnippetCount,
			Name:        "snippet-count",
			Aliases:     []string{"s"},
			Value:       1,
			Usage:       "number of snippets to display",
		},
		&cli.BoolFlag{
			Destination: &internal.IncludeHidden,
			Name:        "hidden",
			Usage:       "include hidden files",
		},
		&cli.StringSliceFlag{
			Destination: internal.AllowListExtensions,
			Name:        "include-ext",
			Aliases:     []string{"i"},
			Usage:       "limit to file extensions (N.B. case sensitive) [comma separated list: e.g. go,java,js,C,cpp]",
		},
		&cli.BoolFlag{
			Destination: &internal.FindRoot,
			Name:        "find-root",
			Aliases:     []string{"r"},
			Usage:       "attempts to find the root of this repository by traversing in reverse looking for .git or .hg",
		},
		&cli.StringSliceFlag{
			Destination: &internal.PathDenylist,
			Name:        "exclude-dir",
			Value:       cli.NewStringSlice(".git", ".hg", ".svn", ".jj"),
			Usage:       "directories to exclude",
		},
		&cli.BoolFlag{
			Destination: &internal.CaseSensitive,
			Name:        "case-sensitive",
			Aliases:     []string{"c"},
			Usage:       "make the search case sensitive",
		},
		&cli.StringSliceFlag{
			Destination: &internal.LocationExcludePattern,
			Name:        "exclude-pattern",
			Aliases:     []string{"x"},
			Usage:       "file and directory locations matching case sensitive patterns will be ignored [comma separated list: e.g. vendor,_test.go]",
		},
		&cli.BoolFlag{
			Destination: &internal.IncludeMinified,
			Name:        "min",
			Usage:       "include minified files",
		},
		&cli.IntFlag{
			Destination: &internal.MinifiedLineByteLength,
			Name:        "min-line-length",
			Value:       255,
			Usage:       "number of bytes per average line for file to be considered minified",
		},
		&cli.Int64Flag{
			Destination: &internal.MaxReadSizeBytes,
			Name:        "max-read-size-bytes",
			Value:       1_000_000,
			Usage:       "number of bytes to read into a file with the remaining content ignored",
		},
		&cli.StringFlag{
			Destination: &internal.Format,
			Name:        "format",
			Aliases:     []string{"f"},
			Value:       "text",
			Usage:       "set output format [text, json, vimgrep]",
		},
		&cli.StringFlag{
			Destination: &internal.Ranker,
			Name:        "ranker",
			Value:       "bm25",
			Usage:       "set ranking algorithm [simple, tfidf, tfidf2, bm25]",
		},
		&cli.StringFlag{
			Destination: &internal.FileOutput,
			Name:        "output",
			Aliases:     []string{"o"},
			Usage:       "output filename (default stdout)",
		},
		&cli.StringFlag{
			Destination: &internal.Directory,
			Name:        "dir",
			Usage:       "directory to search, if not set defaults to current working directory",
		},
	},
	Commands: []*cli.Command{
		{
			Name: "web",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "address",
					Value: ":8080",
					Usage: "address and port to listen to in HTTP mode",
				},
				&cli.StringFlag{
					Name:  "template-search",
					Usage: "path to search template for custom styling",
				},
				&cli.StringFlag{
					Name:  "template-display",
					Usage: "path to display template for custom styling",
				},
			},
			Action: func(ctx *cli.Context) error {
				return internal.StartHttpServer(
					ctx.String("address"),
					ctx.String("template-search"),
					ctx.String("template-display"),
				)
			},
		},
	},
	Before: func(ctx *cli.Context) error {
		internal.DirFilePaths = fun.If(
			strings.TrimSpace(internal.Directory) != "",
			internal.Directory,
			".",
		)
		return nil
	},
	Action: func(ctx *cli.Context) error {
		// If there are arguments we want to print straight out to the console
		// otherwise we should enter interactive tui mode

		if SearchString := ctx.Args().Slice(); len(SearchString) != 0 {
			internal.NewConsoleSearch(SearchString)
			return nil
		}

		return internal.NewTuiSearch()
	},
}

func main() {
	// f, _ := os.Create("profile.pprof")
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()

	log.Logger = log.Level(zerolog.InfoLevel)

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("app crashed")
	}
}
