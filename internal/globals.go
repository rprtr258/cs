package internal

import "github.com/urfave/cli/v2"

// Flags set via the CLI which control how the output is displayed
var (
	// Include minified files
	IncludeMinified = false

	// Number of bytes per average line to determine file is minified
	MinifiedLineByteLength = 255

	// Maximum depth to read into any text file
	MaxReadSizeBytes int64 = 1_000_000

	// Disables .gitignore checks
	IgnoreGitIgnore = false

	// Disables ignore file checks
	IgnoreIgnoreFile = false

	// IncludeBinaryFiles toggles checking for binary files using NUL bytes
	IncludeBinaryFiles = false

	// Format sets the output format of the formatter
	Format = ""

	// Ranker sets which ranking algorithm to use
	Ranker = "bm25" // seems to be the best default

	// FileOutput sets the file that output should be written to
	FileOutput = ""

	// Directory if not empty indicates the user wants to search not from the current location
	Directory = ""

	// PathExclude sets the paths that should be skipped
	PathDenylist cli.StringSlice

	// Allow ignoring files by location
	LocationExcludePattern cli.StringSlice

	// CaseSensitive allows tweaking of case in/sensitive search
	CaseSensitive = false

	// FindRoot flag to check for the root of git or hg when run from a deeper directory and search from there
	FindRoot = false

	// AllowListExtensions is a list of extensions which are whitelisted to be processed
	AllowListExtensions = cli.NewStringSlice()

	// SnippetLength contains many characters out of the file to display in snippets
	SnippetLength int64 = 300

	// SnippetCount is the number of snippets per file to display
	SnippetCount = 1

	// Include hidden files and directories in search
	IncludeHidden = false

	// // SearchTemplate is the location to the search page template
	// SearchTemplate = ""
	// // DisplayTemplate is the location to the display page template
	// DisplayTemplate = ""
)
