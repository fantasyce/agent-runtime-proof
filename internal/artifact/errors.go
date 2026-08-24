package artifact

import "fmt"

type Error struct {
	Reason string
	Err    error
}

func (err *Error) Error() string {
	if err.Err == nil {
		return err.Reason
	}
	return fmt.Sprintf("%s: %v", err.Reason, err.Err)
}

func (err *Error) Unwrap() error { return err.Err }

func domainError(reason string, err error) error {
	return &Error{Reason: reason, Err: err}
}
