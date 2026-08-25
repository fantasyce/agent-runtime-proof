//go:build windows

package witness

import (
	"errors"
	"io"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const procThreadAttributeJobList = 0x0002000d

type windowsSupervisor struct {
	command string
	args    []string
	streams Streams
	mu      sync.Mutex
	started bool
	waited  bool
	pid     int
	process windows.Handle
	job     windows.Handle
	stdin   *os.File
	output  sync.WaitGroup
}

func newSupervisor(command string, arguments []string, streams Streams) (supervisor, error) {
	if command == "" {
		return nil, errors.New("witness supervisor command is empty")
	}
	if streams.Stdout == nil {
		streams.Stdout = io.Discard
	}
	if streams.Stderr == nil {
		streams.Stderr = io.Discard
	}
	return &windowsSupervisor{command: command, args: append([]string{}, arguments...), streams: streams}, nil
}

func (value *windowsSupervisor) Start() error {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.started {
		return errors.New("witness process already started")
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		windows.CloseHandle(job)
		return err
	}
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		windows.CloseHandle(job)
		return err
	}
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		closeFiles(stdinReader, stdinWriter)
		windows.CloseHandle(job)
		return err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		closeFiles(stdinReader, stdinWriter, stdoutReader, stdoutWriter)
		windows.CloseHandle(job)
		return err
	}
	childFiles := []*os.File{stdinReader, stdoutWriter, stderrWriter}
	childHandles := []windows.Handle{windows.Handle(stdinReader.Fd()), windows.Handle(stdoutWriter.Fd()), windows.Handle(stderrWriter.Fd())}
	cleanupAll := func() {
		closeFiles(stdinReader, stdinWriter, stdoutReader, stdoutWriter, stderrReader, stderrWriter)
		windows.CloseHandle(job)
	}
	for _, handle := range childHandles {
		if err := windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
			cleanupAll()
			return err
		}
	}
	attributes, err := windows.NewProcThreadAttributeList(2)
	if err != nil {
		cleanupAll()
		return err
	}
	defer attributes.Delete()
	if err := attributes.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&childHandles[0]), uintptr(len(childHandles))*unsafe.Sizeof(childHandles[0])); err != nil {
		cleanupAll()
		return err
	}
	jobHandles := []windows.Handle{job}
	if err := attributes.Update(procThreadAttributeJobList, unsafe.Pointer(&jobHandles[0]), unsafe.Sizeof(jobHandles[0])); err != nil {
		cleanupAll()
		return err
	}
	application, err := windows.UTF16PtrFromString(value.command)
	if err != nil {
		cleanupAll()
		return err
	}
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{value.command}, value.args...)))
	if err != nil {
		cleanupAll()
		return err
	}
	startup := windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})), Flags: windows.STARTF_USESTDHANDLES,
			StdInput: childHandles[0], StdOutput: childHandles[1], StdErr: childHandles[2],
		},
		ProcThreadAttributeList: attributes.List(),
	}
	var process windows.ProcessInformation
	if err := windows.CreateProcess(application, commandLine, nil, nil, true, windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT, nil, nil, &startup.StartupInfo, &process); err != nil {
		cleanupAll()
		return err
	}
	runtime.KeepAlive(childFiles)
	windows.CloseHandle(process.Thread)
	closeFiles(stdinReader, stdoutWriter, stderrWriter)
	value.started = true
	value.pid = int(process.ProcessId)
	value.process = process.Process
	value.job = job
	value.stdin = stdinWriter
	if value.streams.Stdin == nil {
		_ = stdinWriter.Close()
		value.stdin = nil
	} else {
		go func() {
			_, _ = io.Copy(stdinWriter, value.streams.Stdin)
			_ = stdinWriter.Close()
		}()
	}
	value.output.Add(2)
	go copyAndClose(&value.output, value.streams.Stdout, stdoutReader)
	go copyAndClose(&value.output, value.streams.Stderr, stderrReader)
	return nil
}

func (value *windowsSupervisor) PID() int {
	value.mu.Lock()
	defer value.mu.Unlock()
	return value.pid
}

func (value *windowsSupervisor) Wait() error {
	value.mu.Lock()
	if !value.started || value.process == 0 {
		value.mu.Unlock()
		return errors.New("witness process was not started")
	}
	if value.waited {
		value.mu.Unlock()
		return errors.New("witness process was already waited")
	}
	value.waited = true
	process := value.process
	value.mu.Unlock()
	result, waitErr := windows.WaitForSingleObject(process, windows.INFINITE)
	if waitErr == nil && result != windows.WAIT_OBJECT_0 {
		waitErr = errors.New("unexpected Windows process wait result")
	}
	var exitCode uint32
	if waitErr == nil {
		waitErr = windows.GetExitCodeProcess(process, &exitCode)
	}
	value.mu.Lock()
	job := value.job
	value.job = 0
	value.process = 0
	stdin := value.stdin
	value.stdin = nil
	value.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if job != 0 {
		_ = windows.CloseHandle(job)
	}
	_ = windows.CloseHandle(process)
	value.output.Wait()
	if waitErr != nil {
		return waitErr
	}
	if exitCode != 0 {
		return &exitStatusError{code: int(exitCode)}
	}
	return nil
}

func (value *windowsSupervisor) GracefulStop() error {
	value.mu.Lock()
	stdin := value.stdin
	value.stdin = nil
	started := value.started
	value.mu.Unlock()
	if !started {
		return errors.New("witness process was not started")
	}
	if stdin != nil {
		return stdin.Close()
	}
	return nil
}

func (value *windowsSupervisor) ForceStop() error {
	value.mu.Lock()
	defer value.mu.Unlock()
	if !value.started || value.job == 0 {
		return nil
	}
	return windows.TerminateJobObject(value.job, 1)
}

func (value *windowsSupervisor) ForwardSignal(os.Signal) error { return value.GracefulStop() }

func copyAndClose(group *sync.WaitGroup, destination io.Writer, source *os.File) {
	defer group.Done()
	defer source.Close()
	_, _ = io.Copy(destination, source)
}
