// Copyright 2021 The etcd Authors
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

package integration

import (
	"os"
	"path/filepath"
	"testing"

	grpc_logsettable "github.com/grpc-ecosystem/go-grpc-middleware/logging/settable"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.etcd.io/etcd/pkg/v3/testutil"
	"go.etcd.io/etcd/server/v3/embed"
	"go.uber.org/zap/zaptest"
)

var grpc_logger grpc_logsettable.SettableLoggerV2

func init() {
	grpc_logger = grpc_logsettable.ReplaceGrpcLoggerV2()
}

func BeforeTest(t testutil.TB) {
	testutil.BeforeTest(t)

	grpc_zap.SetGrpcLoggerV2(grpc_logger, zaptest.NewLogger(t).Named("grpc"))

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	os.Chdir(t.TempDir())
	t.Cleanup(func() {
		grpc_logger.Reset()
		os.Chdir(previousWD)
	})

}

func MustAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return abs
}

func NewEmbedConfig(t testing.TB, name string) *embed.Config {
	cfg := embed.NewConfig()
	cfg.Name = name
	cfg.ZapLoggerBuilder = embed.NewZapCoreLoggerBuilder(zaptest.NewLogger(t).Named(cfg.Name), nil, nil)
	cfg.Dir = t.TempDir()
	return cfg
}
