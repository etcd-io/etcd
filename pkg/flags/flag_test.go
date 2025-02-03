// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flags

import (
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestSetFlagsFromEnv(t *testing.T) {
	fs := flag.NewFlagSet("testing", flag.ExitOnError)
	fs.String("a", "", "")
	fs.String("b", "", "")
	fs.String("c", "", "")
	fs.Parse([]string{})

	// flags should be settable using env vars
	t.Setenv("ETCD_A", "foo")
	// and command-line flags
	require.NoError(t, fs.Set("b", "bar"))

	// first verify that flags are as expected before reading the env
	for f, want := range map[string]string{
		"a": "",
		"b": "bar",
	} {
		got := fs.Lookup(f).Value.String()
		require.Equalf(t, want, got, "flag %q=%q, want %q", f, got, want)
	}

	// now read the env and verify flags were updated as expected
	require.NoError(t, SetFlagsFromEnv(zaptest.NewLogger(t), "ETCD", fs))
	for f, want := range map[string]string{
		"a": "foo",
		"b": "bar",
	} {
		got := fs.Lookup(f).Value.String()
		assert.Equalf(t, want, got, "flag %q=%q, want %q", f, got, want)
	}
}

func TestSetFlagsFromEnvBad(t *testing.T) {
	// now verify that an error is propagated
	fs := flag.NewFlagSet("testing", flag.ExitOnError)
	fs.Int("x", 0, "")
	t.Setenv("ETCD_X", "not_a_number")
	assert.Error(t, SetFlagsFromEnv(zaptest.NewLogger(t), "ETCD", fs))
}

func TestSetFlagsFromEnvParsingError(t *testing.T) {
	fs := flag.NewFlagSet("etcd", flag.ContinueOnError)
	var tickMs uint
	fs.UintVar(&tickMs, "heartbeat-interval", 0, "Time (in milliseconds) of a heartbeat interval.")

	t.Setenv("ETCD_HEARTBEAT_INTERVAL", "100 # ms")

	err := SetFlagsFromEnv(zaptest.NewLogger(t), "ETCD", fs)
	for _, v := range []string{"invalid syntax", "parse error"} {
		if strings.Contains(err.Error(), v) {
			err = nil
			break
		}
	}
	require.NoErrorf(t, err, "unexpected error %v", err)
}
