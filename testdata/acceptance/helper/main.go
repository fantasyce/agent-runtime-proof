package main

import (
	"runtime"
	"time"
)

var marker = "default"

func main() {
	for {
		runtime.KeepAlive(marker)
		time.Sleep(time.Second)
	}
}
