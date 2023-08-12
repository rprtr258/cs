package internal

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/boyter/gocodewalker"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/rprtr258/fun/iter"
	"github.com/rs/zerolog/log"
)

var DirFilePaths string

// var searchToFileMatchesCacheMutex = sync.Mutex{}
func FindFiles() chan *gocodewalker.File {
	// Now we need to run through every file closed by the filewalker when done
	fileListQueue := make(chan *gocodewalker.File, 1000)

	if FindRoot {
		DirFilePaths = gocodewalker.FindRepositoryRoot(DirFilePaths)
	}

	fileWalker := gocodewalker.NewFileWalker(DirFilePaths, fileListQueue)
	fileWalker.AllowListExtensions = AllowListExtensions.Value()
	fileWalker.IgnoreIgnoreFile = IgnoreIgnoreFile
	fileWalker.IgnoreGitIgnore = IgnoreGitIgnore
	fileWalker.LocationExcludePattern = LocationExcludePattern.Value()

	go func() { _ = fileWalker.Start() }()

	return fileListQueue
}

// Reads the supplied file into memory, but only up to a certain size
func readFileContent(f *gocodewalker.File) []byte {
	fi, err := os.Lstat(f.Location)
	if err != nil {
		return nil
	}

	// Only read up to point of a file because anything beyond that is probably pointless
	if fi.Size() < MaxReadSizeBytes {
		content, err := os.ReadFile(f.Location)
		if err != nil {
			return nil
		}
		return content
	}

	fil, err := os.Open(f.Location)
	if err != nil {
		return nil
	}
	defer fil.Close()

	byteSlice := make([]byte, MaxReadSizeBytes)
	if _, err = fil.Read(byteSlice); err != nil {
		return nil
	}

	return byteSlice
}

// Given a file to read will read the contents into memory and determine if we should process it
// based on checks such as if its binary or minified
func processFile(f *gocodewalker.File) ([]byte, error) {
	content := readFileContent(f)

	if len(content) == 0 {
		log.Debug().
			Str("file", f.Location).
			Msg("empty file so moving on")
		return nil, errors.New("empty file so moving on")
	}

	// Check if this file is binary by checking for nul byte and if so bail out
	// this is how GNU Grep, git and ripgrep binaryCheck for binary files
	if !IncludeBinaryFiles {
		isBinary := false

		binaryCheck := content
		if len(binaryCheck) > 10_000 {
			binaryCheck = content[:10_000]
		}
		if bytes.IndexByte(binaryCheck, 0) != -1 {
			isBinary = true
		}

		if isBinary {
			log.Debug().
				Str("file", f.Location).
				Msg("file determined to be binary so moving on")
			return nil, errors.New("binary file")
		}
	}

	if !IncludeMinified {
		newlines := iter.Count(
			iter.Filter(
				iter.Values(
					iter.FromSlice(content)),
				func(b byte) bool { return b == '\n' }))

		// Check if this file is minified
		// Check if the file is minified and if so ignore it
		sumLineLength := len(content) - newlines
		averageLineLength := sumLineLength / (newlines + 1)

		if averageLineLength > MinifiedLineByteLength {
			log.Debug().
				Str("file", f.Location).
				Msg("file determined to be minified so moving on")
			return nil, errors.New("file determined to be minified")
		}
	}

	return content, nil
}

// FileReaderWorker reads files from disk in parallel
type FileReaderWorker struct {
	fileCount        int64 // Count of the number of files that have been read
	MaxReadSizeBytes int64
	FuzzyMatch       string
}

func NewFileReaderWorker(fuzzy string) *FileReaderWorker {
	return &FileReaderWorker{
		fileCount:        0,
		MaxReadSizeBytes: 1_000_000, // sensible default of 1MB
		FuzzyMatch:       fuzzy,
	}
}

func (f *FileReaderWorker) GetFileCount() int {
	return int(atomic.LoadInt64(&f.fileCount))
}

// Start is responsible for spinning up jobs
// that read files from disk into memory
func (f *FileReaderWorker) Start(input chan *gocodewalker.File) chan *FileJob {
	var wg sync.WaitGroup
	output := make(chan *FileJob, runtime.NumCPU()) // Files to be read into memory for processing
	for i := 0; i < max(2, runtime.NumCPU()); i++ {
		wg.Add(1)
		go func() {
			for res := range input {
				if f.FuzzyMatch != "" {
					if !fuzzy.MatchFold(f.FuzzyMatch, res.Filename) {
						continue
					}
				}

				if fil, err := processFile(res); err == nil {
					atomic.AddInt64(&f.fileCount, 1)
					output <- &FileJob{
						Filename:       res.Filename,
						Extension:      "",
						Location:       res.Location,
						Content:        fil,
						Bytes:          len(fil),
						Score:          0,
						MatchLocations: map[string]iter.Seq[[2]int]{},
					}
				}
			}
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}
