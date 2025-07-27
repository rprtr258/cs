package main

import (
	"fmt"
	"os"
	"regexp"
	"runtime/pprof"
	"slices"
	"time"

	"github.com/rprtr258/cs/internal/core"
	"github.com/rprtr258/cs/internal/str"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// Simple test comparison between various search methods
func main() {
	if core.Pprof {
		f, err := os.Create("csperf.pprof")
		check(err)
		check(pprof.StartCPUProfile(f))
		defer pprof.StopCPUProfile()
	}

	arg1 := os.Args[1]
	arg2 := os.Args[2]

	b, err := os.ReadFile(arg2)
	if err != nil {
		fmt.Print(err)
		return
	}

	fmt.Println("File length", len(b))

	haystack := string(b)

	var start time.Time
	var elapsed time.Duration

	fmt.Println("\nFindAllIndex (regex)")
	r := regexp.MustCompile(regexp.QuoteMeta(arg1))
	for range 3 {
		start = time.Now()
		all := r.FindAllIndex(b, -1)
		elapsed = time.Since(start)
		fmt.Println("Scan took", elapsed, len(all))
	}

	fmt.Println("\nIndexAll (custom)")
	for range 3 {
		start = time.Now()
		all := slices.Collect(str.IndexAll(haystack, arg1, -1))
		elapsed = time.Since(start)
		fmt.Println("Scan took", elapsed, len(all))
	}

	r = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(arg1))
	fmt.Println("\nFindAllIndex (regex ignore case)")
	for range 3 {
		start = time.Now()
		all := r.FindAllIndex(b, -1)
		elapsed = time.Since(start)
		fmt.Println("Scan took", elapsed, len(all))
	}

	fmt.Println("\nIndexAllIgnoreCaseUnicode (custom)")
	for range 3 {
		start = time.Now()
		all := slices.Collect(str.IndexAllIgnoreCase(haystack, arg1, -1))
		elapsed = time.Since(start)
		fmt.Println("Scan took", elapsed, len(all))
	}
}
