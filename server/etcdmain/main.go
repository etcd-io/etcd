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

package etcdmain

import (
	"fmt"
	"os"

	"github.com/coreos/go-systemd/v22/daemon"
	"go.uber.org/zap"
)

func Main(args []string) {
	// Tips: 根据命名分析，这里是检查程序所支持的操作系统架构
	checkSupportArch()

	if len(args) > 1 {
		// Tips 根据第一个参数设置对应的启动方式,可以看到除默认的启动方式外，特殊的是gateway和grpc-proxy
		// gateway是四层代理，grpc-proxy是七层代理
		cmd := args[1]
		switch cmd {
		case "gateway", "grpc-proxy":
			if err := rootCmd.Execute(); err != nil {
				fmt.Fprint(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}

	startEtcdOrProxyV2(args)
}

func notifySystemd(lg *zap.Logger) {
	if lg == nil {
		lg = zap.NewExample()
	}
	lg.Info("notifying init daemon")
	_, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		lg.Error("failed to notify systemd for readiness", zap.Error(err))
		return
	}
	lg.Info("successfully notified init daemon")
}
