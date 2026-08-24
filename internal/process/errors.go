package process

import "fmt"

type ErrorKind string

const (
	ErrorNotFound        ErrorKind = "not_found"
	ErrorInaccessible    ErrorKind = "inaccessible"
	ErrorIdentityChanged ErrorKind = "identity_changed"
	ErrorInternal        ErrorKind = "internal"
)

type Error struct {
	Kind      ErrorKind
	Operation string
	Err       error
}

func (err *Error) Error() string {
	if err.Err == nil {
		return fmt.Sprintf("%s: %s", err.Operation, err.Kind)
	}
	return fmt.Sprintf("%s: %s: %v", err.Operation, err.Kind, err.Err)
}

func (err *Error) Unwrap() error { return err.Err }
