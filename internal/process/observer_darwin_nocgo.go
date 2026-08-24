//go:build darwin && !cgo

package process

import (
	"context"
	"errors"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

type unsupportedObserver struct{}

func NewObserver() Observer { return unsupportedObserver{} }

func (unsupportedObserver) Snapshot(context.Context, int) (model.Candidate, error) {
	return model.Candidate{}, &Error{Kind: ErrorInternal, Operation: "snapshot", Err: errors.New("Darwin process observation requires cgo")}
}

func (unsupportedObserver) List(context.Context, int) ([]model.Candidate, error) {
	return nil, &Error{Kind: ErrorInternal, Operation: "list processes", Err: errors.New("Darwin process observation requires cgo")}
}

func (unsupportedObserver) Revalidate(context.Context, model.Candidate) error {
	return &Error{Kind: ErrorInternal, Operation: "revalidate process", Err: errors.New("Darwin process observation requires cgo")}
}
