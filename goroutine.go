package redisson

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"unsafe"
)

// https://github.com/golang/net/blob/master/http2/gotrack.go
var goroutineSpace = []byte("goroutine ")

// getId returns the current goroutine's id.
func getId() (uint64, error) {
	bp := littleBuf.Get().(*[]byte)
	defer littleBuf.Put(bp)
	b := *bp
	b = b[:runtime.Stack(b, false)]
	// Parse the 4707 out of "goroutine 4707 ["
	b = bytes.TrimPrefix(b, goroutineSpace)
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		return 0, fmt.Errorf("no space found in %q", b)
	}
	b = b[:i]
	n, err := strconv.ParseUint(unsafe.String(&b[0], len(b)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse goroutine ID out of %q: %v", b, err)
	}
	return n, nil
}

// littleBuf is a pool of 64-byte buffers.
var littleBuf = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 64)
		return &buf
	},
}
