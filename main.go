package main

import (
	"log"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

const Version = "1.3.0"

func main() {
	// f, _ := os.Create("profile.pprof")
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()

	rootCmd := cli.App{
		Name: "cs",
		Usage: `code spelunker (cs) code search.
Version ` + Version + `
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
		Version: Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "address",
				Destination: &Address,
				Value:       ":8080",
				Usage:       "address and port to listen to in HTTP mode",
			},
			&cli.BoolFlag{
				Name:        "http-server",
				Destination: &HttpServer,
				Aliases:     []string{"d"},
				Usage:       "start http server for search",
			},
			&cli.BoolFlag{
				Destination: &IncludeBinaryFiles,
				Name:        "binary",
				Usage:       "set to disable binary file detection and search binary files",
			},
			&cli.BoolFlag{
				Destination: &IgnoreIgnoreFile,
				Name:        "no-ignore",
				Usage:       "disables .ignore file logic",
			},
			&cli.BoolFlag{
				Destination: &IgnoreGitIgnore,
				Name:        "no-gitignore",
				Usage:       "disables .gitignore file logic",
			},
			&cli.Int64Flag{
				Destination: &SnippetLength,
				Name:        "snippet-length",
				Aliases:     []string{"n"},
				Value:       300,
				Usage:       "size of the snippet to display",
			},
			&cli.Int64Flag{
				Destination: &SnippetCount,
				Name:        "snippet-count",
				Aliases:     []string{"s"},
				Value:       1,
				Usage:       "number of snippets to display",
			},
			&cli.BoolFlag{
				Destination: &IncludeHidden,
				Name:        "hidden",
				Usage:       "include hidden files",
			},
			&cli.StringSliceFlag{
				Destination: AllowListExtensions,
				Name:        "include-ext",
				Aliases:     []string{"i"},
				Usage:       "limit to file extensions (N.B. case sensitive) [comma separated list: e.g. go,java,js,C,cpp]",
			},
			&cli.BoolFlag{
				Destination: &FindRoot,
				Name:        "find-root",
				Aliases:     []string{"r"},
				Usage:       "attempts to find the root of this repository by traversing in reverse looking for .git or .hg",
			},
			&cli.StringSliceFlag{
				Destination: &PathDenylist,
				Name:        "exclude-dir",
				Value:       cli.NewStringSlice(".git", ".hg", ".svn", ".jj"),
				Usage:       "directories to exclude",
			},
			&cli.BoolFlag{
				Destination: &CaseSensitive,
				Name:        "case-sensitive",
				Aliases:     []string{"c"},
				Usage:       "make the search case sensitive",
			},
			&cli.StringFlag{
				Destination: &SearchTemplate,
				Name:        "template-search",
				Usage:       "path to search template for custom styling",
			},
			&cli.StringFlag{
				Destination: &DisplayTemplate,
				Name:        "template-display",
				Usage:       "path to display template for custom styling",
			},
			&cli.StringSliceFlag{
				Destination: &LocationExcludePattern,
				Name:        "exclude-pattern",
				Aliases:     []string{"x"},
				Usage:       "file and directory locations matching case sensitive patterns will be ignored [comma separated list: e.g. vendor,_test.go]",
			},
			&cli.BoolFlag{
				Destination: &IncludeMinified,
				Name:        "min",
				Usage:       "include minified files",
			},
			&cli.IntFlag{
				Destination: &MinifiedLineByteLength,
				Name:        "min-line-length",
				Value:       255,
				Usage:       "number of bytes per average line for file to be considered minified",
			},
			&cli.Int64Flag{
				Destination: &MaxReadSizeBytes,
				Name:        "max-read-size-bytes",
				Value:       1_000_000,
				Usage:       "number of bytes to read into a file with the remaining content ignored",
			},
			&cli.StringFlag{
				Destination: &Format,
				Name:        "format",
				Aliases:     []string{"f"},
				Value:       "text",
				Usage:       "set output format [text, json, vimgrep]",
			},
			&cli.StringFlag{
				Destination: &Ranker,
				Name:        "ranker",
				Value:       "bm25",
				Usage:       "set ranking algorithm [simple, tfidf, tfidf2, bm25]",
			},
			&cli.StringFlag{
				Destination: &FileOutput,
				Name:        "output",
				Aliases:     []string{"o"},
				Usage:       "output filename (default stdout)",
			},
			&cli.StringFlag{
				Destination: &Directory,
				Name:        "dir",
				Usage:       "directory to search, if not set defaults to current working directory",
			},
		},
		Action: func(ctx *cli.Context) error {
			SearchString = ctx.Args().Slice()

			dirFilePaths = []string{"."}
			if strings.TrimSpace(Directory) != "" {
				dirFilePaths = []string{Directory}
			}

			// If there are arguments we want to print straight out to the console
			// otherwise we should enter interactive tui mode
			switch {
			case HttpServer:
				// start HTTP server
				StartHttpServer()
			case len(SearchString) != 0:
				NewConsoleSearch()
			default:
				NewTuiSearch()
			}

			return nil
		},
	}

	if err := rootCmd.Run(os.Args); err != nil {
		log.Fatalln(err.Error())
	}
}
