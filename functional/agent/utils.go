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
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/coreos/etcd/pkg/fileutil"
)

// TODO: support separate WAL directory
func archive(baseDir, etcdLogPath, dataDir string) error {
	dir := filepath.Join(baseDir, "etcd-failure-archive", time.Now().Format(time.RFC3339))
	if existDir(dir) {
		dir = filepath.Join(baseDir, "etcd-failure-archive", time.Now().Add(time.Second).Format(time.RFC3339))
	}
	if err := fileutil.TouchDirAll(dir); err != nil {
		return err
	}

	if err := os.Rename(etcdLogPath, filepath.Join(dir, "etcd.log")); err != nil {
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

func stopWithSig(cmd *exec.Cmd, sig os.Signal) error {
	err := cmd.Process.Signal(sig)
	if err != nil {
		return err
	}

	errc := make(chan error)
	go func() {
		_, ew := cmd.Process.Wait()
		errc <- ew
		close(errc)
	}()

	select {
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
	case e := <-errc:
		return e
	}
	err = <-errc
	return err
}

func cleanPageCache() error {
	// https://www.kernel.org/doc/Documentation/sysctl/vm.txt
	// https://github.com/torvalds/linux/blob/master/fs/drop_caches.c
	cmd := exec.Command("/bin/sh", "-c", `echo "echo 1 > /proc/sys/vm/drop_caches" | sudo sh`)
	return cmd.Run()
}
