package processor

import (
	"testing"
)

func TestProcessMatchesSingleMatch(t *testing.T) {
	StopProcessing = false
	ResultLimit = 100
	SearchString = []string{
		"match",
	}

	res := FileJob{
		Locations: map[string][]int{},
	}

	matches := processMatches(&res, "this is a match")

	if matches {
		t.Errorf("Response should be false")
	}

	if res.Score != 1 {
		t.Errorf("Score should be 1")
	}
}

func TestProcessMatchesTwoMatch(t *testing.T) {
	StopProcessing = false
	ResultLimit = 100
	SearchString = []string{
		"match",
		"this",
	}

	res := FileJob{
		Locations: map[string][]int{},
	}

	matches := processMatches(&res, "this is a match")

	if matches {
		t.Errorf("Response should be false")
	}

	if res.Score != 2 {
		t.Errorf("Score should be 1")
	}
}

func TestProcessMatchesTwoAndMatch(t *testing.T) {
	StopProcessing = false
	ResultLimit = 100
	SearchString = []string{
		"match",
		"AND",
		"this",
	}

	res := FileJob{
		Locations: map[string][]int{},
	}

	matches := processMatches(&res, "this is a match")

	if matches {
		t.Errorf("Response should be false")
	}

	if res.Score != 2 {
		t.Errorf("Score should be 1")
	}
}

func TestProcessMatchesTwoNotMatch(t *testing.T) {
	StopProcessing = false
	ResultLimit = 100
	SearchString = []string{
		"match",
		"NOT",
		"this",
	}

	res := FileJob{
		Locations: map[string][]int{},
	}

	matches := processMatches(&res, "this is a match")

	if matches {
		t.Errorf("Response should be false")
	}

	if res.Score != 0 {
		t.Error("Score should be 0 got", res.Score)
	}
}

func TestProcessMatchesFuzzyOne(t *testing.T) {
	StopProcessing = false
	ResultLimit = 100
	SearchString = []string{
		"this~1",
	}

	res := FileJob{
		Locations: map[string][]int{},
	}

	matches := processMatches(&res, "this is a match")

	if matches {
		t.Errorf("Response should be false")
	}

	if res.Score != 4 {
		t.Error("Score should be 4 got", res.Score)
	}
}

func TestProcessMatchesFuzzyTwo(t *testing.T) {
	StopProcessing = false
	ResultLimit = 100
	SearchString = []string{
		"this~2",
	}

	res := FileJob{
		Locations: map[string][]int{},
	}

	matches := processMatches(&res, "this is a match")

	if matches {
		t.Errorf("Response should be false")
	}

	if res.Score != 2 {
		t.Error("Score should be 2 got", res.Score)
	}
}

func TestFileReaderWorker(t *testing.T) {
	ResultLimit = 100

	input := make(chan *FileJob, 10)
	output := make(chan *FileJob, 10)

	input <- &FileJob{
		Filename:  "workers.go",
		Extension: "go",
		Location:  "./workers.go",
		Content:   nil,
		Bytes:     0,
		Hash:      nil,
		Binary:    false,
		Score:     0,
		Locations: nil,
	}
	close(input)

	FileReaderWorker(input, output)

	out := []*FileJob{}
	for o := range output {
		out = append(out, o)
	}

	if len(out) == 0 {
		t.Error("Expected at least one")
	}
}

func TestFileProcessorWorker(t *testing.T) {
	ResultLimit = 100

	input := make(chan *FileJob, 10)
	output := make(chan *FileJob, 10)

	input <- &FileJob{
		Filename:  "workers.go",
		Extension: "go",
		Location:  "./workers.go",
		Content:   []byte("this is some content of stuff"),
		Bytes:     0,
		Hash:      nil,
		Binary:    false,
		Score:     100,
		Locations: map[string][]int{},
	}
	close(input)

	FileProcessorWorker(input, output)

	out := []*FileJob{}
	for o := range output {
		out = append(out, o)
	}

	if len(out) == 0 {
		t.Error("Expected at least one")
	}
}
