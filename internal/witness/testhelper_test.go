package witness

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestWitnessPortableProcessHelper(t *testing.T) {
	separator := -1
	for index, value := range os.Args {
		if value == "--portable-witness-helper" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		return
	}
	switch os.Args[separator+1] {
	case "proxy":
		time.Sleep(100 * time.Millisecond)
		contents, _ := io.ReadAll(os.Stdin)
		_, _ = os.Stdout.Write(contents)
		_, _ = io.WriteString(os.Stderr, "child-stderr")
		code, _ := strconv.Atoi(os.Args[separator+2])
		os.Exit(code)
	case "hang-eof":
		ignoreHelperTermination()
		_, _ = io.ReadAll(os.Stdin)
		for {
			time.Sleep(time.Hour)
		}
	case "ready-hang":
		ignoreHelperTermination()
		fmt.Fprintln(os.Stdout, "ready")
		for {
			time.Sleep(time.Hour)
		}
	}
}

func portableHelperArguments(mode string, extra ...string) []string {
	return append([]string{"-test.run=TestWitnessPortableProcessHelper", "--", "--portable-witness-helper", mode}, extra...)
}
