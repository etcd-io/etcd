package refactoring

import "errors"

type (
	ErrInvalidArgs      error
	ErrInvalidSelection error
)

func errInvalidArgs(reason string) ErrInvalidArgs {
	return errors.New("invalid arguments: " + reason)
}

func errInvalidSelection(reason string) ErrInvalidSelection {
	return errors.New("invalid selection: " + reason)
}
