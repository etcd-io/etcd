package utils

import (
	"github.com/pingcap/errors"
)

//OriginError return original err
func OriginError(err error) error {
	for {
		e := errors.Cause(err)
		if e == err {
			break
		}
		err = e
	}
	return err
}
