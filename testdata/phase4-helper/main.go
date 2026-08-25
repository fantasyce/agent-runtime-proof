package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"syscall"
	"time"

	proofvalidator "github.com/fantasyce/agent-runtime-proof/internal/proof"
)

func main() {
	if len(os.Args) < 2 {
		fail("missing helper command")
	}
	switch os.Args[1] {
	case "serve":
		serve()
	case "measure-mcp":
		if len(os.Args) != 4 {
			fail("usage: measure-mcp CANDIDATE ITERATIONS")
		}
		iterations, err := strconv.Atoi(os.Args[3])
		if err != nil || iterations < 1 || iterations > 100 {
			fail("invalid iteration count")
		}
		measureMCP(os.Args[2], iterations)
	case "verify-mcp":
		if len(os.Args) != 6 {
			fail("usage: verify-mcp CANDIDATE EXPECTATION PID VERDICT")
		}
		pid, err := strconv.Atoi(os.Args[4])
		if err != nil || pid <= 0 {
			fail("invalid PID")
		}
		verifyMCP(os.Args[2], os.Args[3], pid, os.Args[5])
	case "hold-mcp":
		if len(os.Args) != 3 && len(os.Args) != 4 {
			fail("usage: hold-mcp CANDIDATE [STOP_FILE]")
		}
		stopFile := ""
		if len(os.Args) == 4 {
			stopFile = os.Args[3]
		}
		holdMCP(os.Args[2], stopFile)
	case "verify-proof-file":
		if len(os.Args) != 3 {
			fail("usage: verify-proof-file PROOF_JSON")
		}
		document, err := os.ReadFile(os.Args[2])
		if err != nil {
			fail("could not read Proof")
		}
		if _, err := proofvalidator.Validate(document); err != nil {
			fail("Proof schema or content ID validation failed")
		}
		fmt.Println("PROOF_VALID=1")
	default:
		fail("unknown helper command")
	}
}

func serve() {
	fmt.Printf("READY PID=%d\n", os.Getpid())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
}

func measureMCP(candidate string, iterations int) {
	durations := make([]time.Duration, 0, iterations)
	var maximumRSS int64
	for index := 0; index < iterations; index++ {
		started := time.Now()
		command := exec.Command(candidate, "mcp")
		stdin, err := command.StdinPipe()
		if err != nil {
			fail("could not create MCP stdin")
		}
		stdout, err := command.StdoutPipe()
		if err != nil {
			fail("could not create MCP stdout")
		}
		var stderr bytes.Buffer
		command.Stderr = &stderr
		if err := command.Start(); err != nil {
			fail("could not start MCP candidate")
		}
		request := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"phase4-measure","version":"1"}}}` + "\n" +
			`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
		if _, err := stdin.Write([]byte(request)); err != nil {
			fail("could not write MCP request")
		}
		reader := bufio.NewReader(stdout)
		if _, err := reader.ReadBytes('\n'); err != nil {
			fail("missing initialize response")
		}
		line, err := reader.ReadBytes('\n')
		if err != nil {
			fail("missing tools response")
		}
		var response struct {
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if json.Unmarshal(line, &response) != nil || len(response.Result.Tools) != 3 {
			fail("unexpected MCP tool list")
		}
		_ = stdin.Close()
		if err := command.Wait(); err != nil || stderr.Len() != 0 {
			fail("MCP candidate did not exit cleanly")
		}
		durations = append(durations, time.Since(started))
		if rss := peakRSS(command.ProcessState); rss > maximumRSS {
			maximumRSS = rss
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[(len(durations)*95+99)/100-1]
	fmt.Printf("ITERATIONS=%d P95_MS=%d MAX_RSS_BYTES=%d\n", iterations, p95.Milliseconds(), maximumRSS)
	if p95 > 2*time.Second {
		fail("MCP startup p95 exceeded two seconds")
	}
	if maximumRSS > 150*1024*1024 {
		fail("MCP peak RSS exceeded 150 MiB")
	}
}

func verifyMCP(candidate, expectation string, pid int, verdict string) {
	session := startMCP(candidate)
	defer session.stop()
	response := session.call(3, "tools/call", map[string]any{"name": "verify_local_runtime", "arguments": map[string]any{"pid": pid, "expectation_path": expectation}})
	result, _ := response["result"].(map[string]any)
	structured, _ := result["structuredContent"].(map[string]any)
	proof, _ := structured["proof"].(map[string]any)
	if proof["verdict"] != verdict {
		fail("unexpected MCP verdict")
	}
	proofID, _ := proof["proof_id"].(string)
	if len(proofID) != 71 || proofID[:7] != "sha256:" {
		fail("invalid MCP Proof ID")
	}
	encodedProof, _ := json.Marshal(proof)
	if _, err := proofvalidator.Validate(encodedProof); err != nil {
		fail("MCP Proof schema or content ID validation failed")
	}
	fmt.Printf("VERDICT=%s PROOF_ID=%s TOOLS=3\n", verdict, proofID)
}

func holdMCP(candidate, stopFile string) {
	session := startMCP(candidate)
	fmt.Printf("READY PID=%d\n", session.command.Process.Pid)
	if stopFile == "" {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		<-signals
	} else {
		for {
			if _, err := os.Stat(stopFile); err == nil {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	session.stop()
}

type mcpSession struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	reader  *bufio.Reader
	stderr  *bytes.Buffer
}

func startMCP(candidate string) *mcpSession {
	command := exec.Command(candidate, "mcp")
	stdin, err := command.StdinPipe()
	if err != nil {
		fail("could not create MCP stdin")
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		fail("could not create MCP stdout")
	}
	stderr := new(bytes.Buffer)
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		fail("could not start MCP candidate")
	}
	session := &mcpSession{command: command, stdin: stdin, reader: bufio.NewReader(stdout), stderr: stderr}
	initialized := session.call(1, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "phase4-helper", "version": "1"}})
	if initialized["error"] != nil {
		fail("MCP initialize failed")
	}
	session.notify("notifications/initialized", map[string]any{})
	listed := session.call(2, "tools/list", map[string]any{})
	result, _ := listed["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 3 {
		fail("unexpected MCP tool list")
	}
	return session
}

func (session *mcpSession) call(id int, method string, params map[string]any) map[string]any {
	encoded, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if _, err := session.stdin.Write(append(encoded, '\n')); err != nil {
		fail("could not write MCP request")
	}
	line, err := session.reader.ReadBytes('\n')
	if err != nil {
		fail("missing MCP response")
	}
	var response map[string]any
	if json.Unmarshal(line, &response) != nil {
		fail("invalid MCP response")
	}
	return response
}

func (session *mcpSession) notify(method string, params map[string]any) {
	encoded, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if _, err := session.stdin.Write(append(encoded, '\n')); err != nil {
		fail("could not write MCP notification")
	}
}

func (session *mcpSession) stop() {
	if session.stdin == nil {
		return
	}
	_ = session.stdin.Close()
	if err := session.command.Wait(); err != nil || session.stderr.Len() != 0 {
		fail("MCP candidate did not exit cleanly")
	}
	session.stdin = nil
}

func fail(message string) { fmt.Fprintln(os.Stderr, "phase4-helper:", message); os.Exit(1) }
