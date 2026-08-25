package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/receipt"
	sdkmodel "github.com/fantasyce/agent-runtime-proof/sdk/model"
	sdkwitness "github.com/fantasyce/agent-runtime-proof/sdk/witness"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(64)
	}
	switch os.Args[1] {
	case "echo":
		time.Sleep(200 * time.Millisecond)
		contents, _ := io.ReadAll(os.Stdin)
		_, _ = os.Stdout.Write(contents)
		_, _ = io.WriteString(os.Stderr, "child-stderr\n")
	case "exit":
		time.Sleep(200 * time.Millisecond)
		code, err := strconv.Atoi(os.Args[2])
		if err != nil {
			os.Exit(64)
		}
		os.Exit(code)
	case "hang-eof":
		ignoreTermination()
		_, _ = io.ReadAll(os.Stdin)
		for {
			time.Sleep(time.Hour)
		}
	case "term":
		termination := terminationChannel()
		fmt.Fprintln(os.Stdout, "ready")
		<-termination
		fmt.Fprintln(os.Stdout, "terminated")
	case "tree":
		child := exec.Command(os.Args[0], "child")
		if err := child.Start(); err != nil {
			os.Exit(70)
		}
		fmt.Fprintf(os.Stdout, "target_pid=%d child_pid=%d\n", os.Getpid(), child.Process.Pid)
		_ = child.Wait()
	case "child":
		ignoreTermination()
		for {
			time.Sleep(time.Hour)
		}
	case "validate-receipt":
		validateReceipt()
	case "sdk-prepare":
		validateSDK(false)
	case "sdk-spawn":
		validateSDK(true)
	case "validate-expectation":
		if len(os.Args) != 3 {
			os.Exit(64)
		}
		if _, err := expectation.Load(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		os.Exit(64)
	}
}

func validateSDK(spawn bool) {
	if len(os.Args) < 6 {
		os.Exit(64)
	}
	controller, err := sdkwitness.New(sdkwitness.Config{
		Home: os.Args[2],
		Tool: sdkmodel.ToolInfo{Name: "agent-runtime-proof", Version: "0.3.0-phase3", Commit: "abcdef0", Toolchain: "go1.27.0"},
	})
	var prepared *sdkwitness.PreparedLaunch
	if err == nil {
		prepared, err = controller.PrepareLaunch(context.Background(), sdkwitness.Request{ExpectationPath: os.Args[3], Command: os.Args[4:]})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if spawn {
		command, arguments := prepared.Command()
		reader, writer, pipeErr := os.Pipe()
		if pipeErr != nil {
			os.Exit(1)
		}
		child := exec.Command(command, arguments...)
		child.Stdin = reader
		child.Stdout = io.Discard
		child.Stderr = io.Discard
		if err = child.Start(); err == nil {
			_, err = prepared.Spawned(context.Background(), child.Process.Pid)
		}
		_ = writer.Close()
		_ = reader.Close()
		if child.Process != nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "SDK_SPAWN=PASS")
		return
	}
	fmt.Fprintln(os.Stdout, "SDK_PREPARE=PASS")
}

func validateReceipt() {
	if len(os.Args) != 5 {
		os.Exit(64)
	}
	document, err := os.ReadFile(os.Args[2])
	if err != nil {
		os.Exit(1)
	}
	value, err := receipt.Validate(document)
	if err != nil {
		os.Exit(2)
	}
	for _, prohibited := range []string{os.Args[3], os.Args[4]} {
		if prohibited != "" && bytes.Contains(document, []byte(prohibited)) {
			os.Exit(3)
		}
	}
	var raw map[string]any
	if err := json.Unmarshal(document, &raw); err != nil {
		os.Exit(4)
	}
	for _, forbidden := range []string{"argv", "environment", "executable_path"} {
		if _, exists := raw[forbidden]; exists {
			os.Exit(5)
		}
	}
	if !strings.HasPrefix(value.ReceiptID, "sha256:") || value.Process.PID <= 0 {
		os.Exit(6)
	}
	fmt.Printf("RECEIPT_ID=%s\nRECEIPT_PID=%d\n", value.ReceiptID, value.Process.PID)
}
