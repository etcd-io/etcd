package store

import (
	"fmt"
	)

type NotFoundError string

func (e NotFoundError) Error() string {
	return fmt.Sprintf("Key %s Not Found", string(e))
}

type NotFile string

func (e NotFile) Error() string {
	return fmt.Sprintf("Try to set value to a dir %s", string(e))
}

type TestFail string

func (e TestFail) Error() string {
	return fmt.Sprintf("Test %s fails", string(e))
}