package cache

import (
	"errors"
)

// IsErrMissing allows to access to the internal error.
// TODO(ldez) the handling of this error inside runner_action.go should be refactored.
func IsErrMissing(err error) bool {
	var errENF *entryNotFoundError
	return errors.As(err, &errENF)
}
