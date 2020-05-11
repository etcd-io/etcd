// Manual code for validation tests.

package mwitkow_testproto

import "errors"

func (p *PingRequest) Validate() error {
	if p.SleepTimeMs > 10000 {
		return errors.New("cannot sleep for more than 10s")
	}
	return nil
}
