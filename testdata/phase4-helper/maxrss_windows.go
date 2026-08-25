//go:build windows

package main

import "os"

func peakRSS(*os.ProcessState) int64 { return 0 }
