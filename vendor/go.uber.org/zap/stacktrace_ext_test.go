// Copyright (c) 2016, 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package zap_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// _zapPackages are packages that we search for in the logging output to match a
// zap stack frame. It is different from _zapStacktracePrefixes which  is only
// intended to match on the function name, while this is on the full output
// which includes filenames.
var _zapPackages = []string{
	"go.uber.org/zap.",
	"go.uber.org/zap/zapcore.",
}

func TestStacktraceFiltersZapLog(t *testing.T) {
	withLogger(t, func(logger *zap.Logger, out *bytes.Buffer) {
		logger.Error("test log")
		logger.Sugar().Error("sugar test log")

		require.Contains(t, out.String(), "TestStacktraceFiltersZapLog", "Should not strip out non-zap import")
		verifyNoZap(t, out.String())
	})
}

func TestStacktraceFiltersZapMarshal(t *testing.T) {
	withLogger(t, func(logger *zap.Logger, out *bytes.Buffer) {
		marshal := func(enc zapcore.ObjectEncoder) error {
			logger.Warn("marshal caused warn")
			enc.AddString("f", "v")
			return nil
		}
		logger.Error("test log", zap.Object("obj", zapcore.ObjectMarshalerFunc(marshal)))

		logs := out.String()

		// The marshal function (which will be under the test function) should not be stripped.
		const marshalFnPrefix = "TestStacktraceFiltersZapMarshal."
		require.Contains(t, logs, marshalFnPrefix, "Should not strip out marshal call")

		// There should be no zap stack traces before that point.
		marshalIndex := strings.Index(logs, marshalFnPrefix)
		verifyNoZap(t, logs[:marshalIndex])

		// After that point, there should be zap stack traces - we don't want to strip out
		// the Marshal caller information.
		for _, fnPrefix := range _zapPackages {
			require.Contains(t, logs[marshalIndex:], fnPrefix, "Missing zap caller stack for Marshal")
		}
	})
}

func TestStacktraceFiltersVendorZap(t *testing.T) {
	// We need to simulate a zap as a vendor library, so we're going to create a fake GOPATH
	// and run the above test which will contain zap in the vendor directory.
	withGoPath(t, func(goPath string) {
		curDir, err := os.Getwd()
		require.NoError(t, err, "Failed to get current directory")

		testDir := filepath.Join(goPath, "src/go.uber.org/zap_test/")
		vendorDir := filepath.Join(testDir, "vendor")
		require.NoError(t, os.MkdirAll(testDir, 0777), "Failed to create source director")

		curFile := getSelfFilename(t)
		//copyFile(t, curFile, filepath.Join(testDir, curFile))
		setupSymlink(t, curFile, filepath.Join(testDir, curFile))

		// Set up symlinks for zap, and for any test dependencies.
		setupSymlink(t, curDir, filepath.Join(vendorDir, "go.uber.org/zap"))
		for _, testDep := range []string{"github.com/stretchr/testify"} {
			target := filepath.Join(curDir, "vendor", testDep)
			_, err := os.Stat(target)
			require.NoError(t, err, "Required dependency (%v) not installed in vendor", target)
			setupSymlink(t, target, filepath.Join(vendorDir, testDep))
		}

		// Now run the above test which ensures we filter out zap stacktraces, but this time
		// zap is in a vendor
		cmd := exec.Command("go", "test", "-v", "-run", "TestStacktraceFiltersZap")
		cmd.Dir = testDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to run test in vendor directory, output: %s", out)
		assert.Contains(t, string(out), "PASS")
	})
}

// withLogger sets up a logger with a real encoder set up, so that any marshal functions are called.
// The inbuilt observer does not call Marshal for objects/arrays, which we need for some tests.
func withLogger(t *testing.T, fn func(logger *zap.Logger, out *bytes.Buffer)) {
	buf := &bytes.Buffer{}
	encoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	logger := zap.New(core, zap.AddStacktrace(zap.DebugLevel))
	fn(logger, buf)
}

func verifyNoZap(t *testing.T, logs string) {
	for _, fnPrefix := range _zapPackages {
		require.NotContains(t, logs, fnPrefix, "Should not strip out marshal call")
	}
}

func withGoPath(t *testing.T, f func(goPath string)) {
	goPath, err := ioutil.TempDir("", "gopath")
	require.NoError(t, err, "Failed to create temporary directory for GOPATH")
	//defer os.RemoveAll(goPath)

	os.Setenv("GOPATH", goPath)
	defer os.Setenv("GOPATH", os.Getenv("GOPATH"))

	f(goPath)
}

func getSelfFilename(t *testing.T) string {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "Failed to get caller information to identify local file")

	return filepath.Base(file)
}

func setupSymlink(t *testing.T, src, dst string) {
	// Make sure the destination directory exists.
	os.MkdirAll(filepath.Dir(dst), 0777)

	// Get absolute path of the source for the symlink, otherwise we can create a symlink
	// that uses relative paths.
	srcAbs, err := filepath.Abs(src)
	require.NoError(t, err, "Failed to get absolute path")

	require.NoError(t, os.Symlink(srcAbs, dst), "Failed to set up symlink")
}
