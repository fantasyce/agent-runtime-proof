//go:build windows

package store

import (
	"os"
	"testing"
)

func assertStoredReceiptPermissions(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("receipt mode = %v, err=%v", info.Mode(), err)
	}
}
