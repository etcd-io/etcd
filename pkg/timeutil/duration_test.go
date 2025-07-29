package timeutil

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnmarshallJSON(t *testing.T) {
	data := []byte(`"10s"`)
	var d Duration
	json.Unmarshal(data, &d)
	require.Equal(t, d.Duration, 10*time.Second)

	data = []byte("10_000_000_000")
	json.Unmarshal(data, &d)
	require.Equal(t, d.Duration, 10*time.Second)
}

func TestFlagValueInterface(t *testing.T) {
	var d Duration
	d.Set("10s")
	require.Equal(t, d.Duration, 10*time.Second)
	require.Equal(t, d.String(), "10s")

}
