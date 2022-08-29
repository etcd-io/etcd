// Copyright 2018 The etcd Authors
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

package agent

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"

	"go.uber.org/zap"
)

// TODO: support separate WAL directory
func archive(lg *zap.Logger, baseDir, etcdLogPath, dataDir string) error {
	dir := filepath.Join(baseDir, "etcd-failure-archive", time.Now().Format(time.RFC3339))
	if existDir(dir) {
		dir = filepath.Join(baseDir, "etcd-failure-archive", time.Now().Add(time.Second).Format(time.RFC3339))
	}
	if err := fileutil.TouchDirAll(lg, dir); err != nil {
		return err
	}

	dst := filepath.Join(dir, "etcd.log")
	if err := copyFile(etcdLogPath, dst); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Rename(dataDir, filepath.Join(dir, filepath.Base(dataDir))); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func existDir(fpath string) bool {
	st, err := os.Stat(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	} else {
		return st.IsDir()
	}
	return false
}

func getURLAndPort(addr string) (urlAddr *url.URL, port int, err error) {
	urlAddr, err = url.Parse(addr)
	if err != nil {
		return nil, -1, err
	}
	var s string
	_, s, err = net.SplitHostPort(urlAddr.Host)
	if err != nil {
		return nil, -1, err
	}
	port, err = strconv.Atoi(s)
	if err != nil {
		return nil, -1, err
	}
	return urlAddr, port, err
}

func copyFile(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer w.Close()

	if _, err = io.Copy(w, f); err != nil {
		return err
	}
	return w.Sync()
}

func safeDataToFile(filePath string, fileData []byte, mode os.FileMode) error {
	if filePath != "" {
		if len(fileData) == 0 {
			return fmt.Errorf("got empty data for %q", filePath)
		}
		if err := os.WriteFile(filePath, fileData, mode); err != nil {
			return fmt.Errorf("writing file %q failed, %w", filePath, err)
		}
	}
	return nil
}

func loadFileData(filePath string) ([]byte, error) {
	if !fileutil.Exist(filePath) {
		return nil, fmt.Errorf("cannot find %q", filePath)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %q failed, %w", filePath, err)
	}
	return data, nil
}

func checkTCPConnect(lg *zap.Logger, target string) error {
	for i := 0; i < 10; i++ {
		if conn, err := net.Dial("tcp", target); err != nil {
			lg.Error("The target isn't reachable", zap.Int("retries", i), zap.String("target", target), zap.Error(err))
		} else {
			if conn != nil {
				conn.Close()
				lg.Info("The target is reachable", zap.Int("retries", i), zap.String("target", target))
				return nil
			}
			lg.Error("The target isn't reachable due to the returned conn is nil", zap.Int("retries", i), zap.String("target", target))
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timed out waiting for the target (%s) to be reachable", target)
}

func cleanPageCache() error {
	// https://www.kernel.org/doc/Documentation/sysctl/vm.txt
	// https://github.com/torvalds/linux/blob/master/fs/drop_caches.c
	cmd := exec.Command("/bin/sh", "-c", `echo "echo 1 > /proc/sys/vm/drop_caches" | sudo -s -n`)
	return cmd.Run()
}
