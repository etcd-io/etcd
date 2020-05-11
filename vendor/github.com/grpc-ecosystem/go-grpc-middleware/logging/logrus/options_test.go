package grpc_logrus

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestDurationToTimeMillisField(t *testing.T) {
	_, val := DurationToTimeMillisField(time.Microsecond * 100)
	assert.Equal(t, val.(float32), float32(0.1), "sub millisecond values should be correct")
}
