package internal

import "github.com/rprtr258/fun/iter"

// FileJob - results of processing internally before sent to the formatter
type FileJob struct {
	Filename       string
	Extension      string
	Location       string
	Content        []byte
	Bytes          int
	Score          float64
	MatchLocations map[string]iter.Seq[[2]int]
}
