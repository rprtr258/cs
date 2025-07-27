package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"
)

// Returns the current time as a millisecond timestamp
func NowMillis() int64 {
	return time.Now().UnixMilli()
}

// Returns the current time as a nanosecond timestamp as some things
// are far too fast to measure using nanoseconds
func NowNanos() int64 {
	return time.Now().UnixNano()
}

// CreateFmts Create a random str to define where the start and end of
// out highlight should be which we swap out later after we have
// HTML escaped everything
func CreateFmts() (string, string) {
	md5Digest := md5.New()
	fmtBegin := hex.EncodeToString(md5Digest.Sum(fmt.Appendf(nil, "begin_%d", NowNanos())))
	fmtEnd := hex.EncodeToString(md5Digest.Sum(fmt.Appendf(nil, "end_%d", NowNanos())))
	return fmtBegin, fmtEnd
}

// Abs returns the absolute value of x.
func Abs(x int) int {
	return max(x, -x)
}

func DigitsCount(n int) int {
	res := 0
	for n > 0 {
		n /= 10
		res++
	}
	return res
}
