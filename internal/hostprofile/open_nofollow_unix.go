//go:build !windows

package hostprofile

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

func readPinnedConfig(ctx context.Context, path string, maximum int64) ([]byte, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) || maximum <= 0 {
		return nil, errors.New("invalid configuration path")
	}
	if runtime.GOOS == "darwin" && runtimePathPrefix(cleaned, "/var/") {
		cleaned = "/private" + cleaned
	}
	if runtime.GOOS == "darwin" && runtimePathPrefix(cleaned, "/tmp/") {
		cleaned = "/private" + cleaned
	}
	current, err := unix.Open("/", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	owned := true
	defer func() {
		if owned {
			_ = unix.Close(current)
		}
	}()
	components := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	for index, component := range components {
		if component == "" || component == "." || component == ".." {
			return nil, errors.New("invalid configuration path")
		}
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
		if index < len(components)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, openErr := unix.Openat(current, component, flags, 0)
		if openErr != nil {
			return nil, openErr
		}
		_ = unix.Close(current)
		current = next
	}
	file := os.NewFile(uintptr(current), "host-config")
	owned = false
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return nil, errors.New("configuration is not a regular file")
	}
	reader := io.LimitReader(file, maximum+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, errors.New("configuration exceeds limit")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errConfigChanged
	}
	confirmed, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || !bytes.Equal(data, confirmed) {
		return nil, errConfigChanged
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, errConfigChanged
	}
	return data, nil
}

func runtimePathPrefix(value, prefix string) bool {
	return value == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(value, prefix)
}
