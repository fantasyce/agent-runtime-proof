package witness

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"time"
)

const (
	defaultGracePeriod = 5 * time.Second
	maximumGracePeriod = time.Minute
)

type RunRequest struct {
	Command         []string
	ExpectationPath string
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	GracePeriod     time.Duration
}

type Result struct {
	ExitCode  int
	ReceiptID string
	PID       int
}

func Run(ctx context.Context, controller *Controller, request RunRequest) (Result, error) {
	if controller == nil {
		return Result{}, errors.New("witness controller is unavailable")
	}
	gracePeriod := request.GracePeriod
	if gracePeriod == 0 {
		gracePeriod = defaultGracePeriod
	}
	if gracePeriod < 0 || gracePeriod > maximumGracePeriod {
		return Result{}, invalidInput("grace period is invalid")
	}
	prepared, err := controller.PrepareLaunch(ctx, Request{Command: request.Command, ExpectationPath: request.ExpectationPath})
	if err != nil {
		return Result{}, err
	}
	childInput, parentInput, err := os.Pipe()
	if err != nil {
		return Result{}, errors.New("witness stdin pipe is unavailable")
	}
	defer childInput.Close()
	defer parentInput.Close()
	command, arguments := prepared.Command()
	managed, err := newSupervisor(command, arguments, Streams{Stdin: childInput, Stdout: request.Stdout, Stderr: request.Stderr})
	if err != nil {
		return Result{}, errors.New("witness process supervisor is unavailable")
	}
	if err := managed.Start(); err != nil {
		return Result{}, errors.New("witness process could not be started")
	}
	pid := managed.PID()
	receiptValue, err := prepared.Spawned(ctx, pid)
	if err != nil {
		_ = managed.ForceStop()
		_ = managed.Wait()
		return Result{PID: pid}, err
	}
	stdinDone := make(chan struct{}, 1)
	go func() {
		if request.Stdin != nil {
			_, _ = io.Copy(parentInput, request.Stdin)
		}
		_ = parentInput.Close()
		stdinDone <- struct{}{}
	}()
	waitDone := make(chan error, 1)
	go func() { waitDone <- managed.Wait() }()
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, terminationSignals()...)
	defer signal.Stop(signalChannel)
	result := Result{ReceiptID: receiptValue.ReceiptID, PID: pid}
	contextDone := ctx.Done()
	var timer *time.Timer
	var timeout <-chan time.Time
	stopping := false
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case waitErr := <-waitDone:
			code := processExitCode(waitErr)
			if code < 0 {
				return result, errors.New("witness child exit status is unavailable")
			}
			result.ExitCode = code
			return result, nil
		case <-stdinDone:
			stdinDone = nil
			if !stopping {
				stopping = true
				timer = time.NewTimer(gracePeriod)
				timeout = timer.C
			}
		case <-contextDone:
			contextDone = nil
			if !stopping {
				stopping = true
				_ = managed.GracefulStop()
				timer = time.NewTimer(gracePeriod)
				timeout = timer.C
			}
		case forwarded := <-signalChannel:
			if !stopping {
				stopping = true
				_ = managed.ForwardSignal(forwarded)
				timer = time.NewTimer(gracePeriod)
				timeout = timer.C
			}
		case <-timeout:
			timeout = nil
			_ = managed.ForceStop()
		}
	}
}
