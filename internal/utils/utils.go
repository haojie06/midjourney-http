package utils

import (
	"fmt"
	"runtime"
)

func GetGoroutineID() int64 {
	buf := make([]byte, 64)
	n := runtime.Stack(buf, false)
	id := int64(-1)
	fmt.Sscanf(string(buf[:n]), "goroutine %d ", &id)
	return id
}
