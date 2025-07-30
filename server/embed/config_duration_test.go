package embed

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalJSON(t *testing.T) {
	data := []byte(`"10s"`)
	var d Duration
	json.Unmarshal(data, &d)
	require.Equal(t, 10*time.Second, d.Duration)

	data = []byte("10_000_000_000")
	json.Unmarshal(data, &d)
	require.Equal(t, 10*time.Second, d.Duration)
}
