package etcd

import (
	"encoding/json"
	"fmt"
)

type EtcdError struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
	Cause     string `json:"cause,omitempty"`
}

func (e EtcdError) Error() string {
	return fmt.Sprintf("%d: %s (%s)", e.ErrorCode, e.Message, e.Cause)
}

func handleError(b []byte) error {
	var err EtcdError

	json.Unmarshal(b, &err)

	return err
}
