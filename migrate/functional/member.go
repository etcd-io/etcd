// Copyright 2015 CoreOS, Inc.
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

package functional

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Instance struct {
	*exec.Cmd
	Name    string
	DataDir string
	URL     string
	PeerURL string

	stderr io.ReadCloser
}

func NewDefaultInstance(path string) *Instance {
	var args []string
	dir, err := ioutil.TempDir(os.TempDir(), "etcd")
	if err != nil {
		fmt.Printf("unexpected TempDir error: %v", err)
		os.Exit(1)
	}
	args = append(args, "--data-dir="+dir)
	args = append(args, "--name=default")
	i := &Instance{
		Cmd:     exec.Command(path, args...),
		Name:    "default",
		DataDir: dir,
		URL:     "http://127.0.0.1:4001",
		PeerURL: "http://127.0.0.1:7001",
	}
	// always expect to use start_desired_verson mode
	i.Env = append(i.Env,
		"ETCD_START_DESIRED_VERSION=true",
		"ETCD_BINARY_DIR="+binDir,
	)
	return i
}

func (i *Instance) SetV2PeerURL(url string) {
	i.Args = append(i.Args,
		"-listen-peer-urls="+url,
		"-initial-advertise-peer-urls="+url,
		"-initial-cluster",
		i.Name+"="+url,
	)
	i.PeerURL = url
}

func (i *Instance) SetV1PeerAddr(addr string) {
	i.Args = append(i.Args,
		"-peer-addr="+addr,
	)
	i.PeerURL = "http://" + addr
}

func (i *Instance) SetV1Addr(addr string) {
	i.Args = append(i.Args,
		"-addr="+addr,
	)
	i.URL = "http://" + addr
}

func (i *Instance) SetV1Peers(peers []string) {
	i.Args = append(i.Args,
		"-peers="+strings.Join(peers, ","),
	)
}

func (i *Instance) SetName(name string) {
	i.Args = append(i.Args,
		"-name="+name,
	)
	i.Name = name
}

func (i *Instance) SetSnapCount(cnt int) {
	i.Args = append(i.Args,
		"-snapshot-count="+strconv.Itoa(cnt),
	)
}

func (i *Instance) SetDiscovery(url string) {
	i.Args = append(i.Args,
		"-discovery="+url,
	)
}

func (i *Instance) CleanUnsuppportedV1Flags() {
	var args []string
	for _, arg := range i.Args {
		if !strings.HasPrefix(arg, "-peers=") {
			args = append(args, arg)
		}
	}
	i.Args = args
}

func (i *Instance) Clone(path string) *Instance {
	return &Instance{
		Cmd: &exec.Cmd{
			Path: path,
			Args: i.Args,
			Env:  i.Env,
			Dir:  i.Dir,
		},
		Name:    i.Name,
		DataDir: i.DataDir,
		URL:     i.URL,
		PeerURL: i.PeerURL,
	}
}

func (i *Instance) Start() error {
	var err error
	if i.stderr, err = i.Cmd.StderrPipe(); err != nil {
		return err
	}
	if err := i.Cmd.Start(); err != nil {
		return err
	}
	for k := 0; k < 50; k++ {
		_, err := http.Get(i.URL)
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	errMsg, _ := ioutil.ReadAll(i.stderr)
	return fmt.Errorf("instance %s failed to be available after a long time: %s", i.Name, errMsg)
}

func (i *Instance) Stop() {
	if err := i.Cmd.Process.Kill(); err != nil {
		fmt.Printf("Process Kill error: %v", err)
		return
	}
	ioutil.ReadAll(i.stderr)
	i.Cmd.Wait()
}

func (i *Instance) Restart() error {
	i.Stop()
	return i.Start()
}

func (i *Instance) Terminate() {
	i.Stop()
	os.RemoveAll(i.DataDir)
}

type Instances []*Instance

func NewDefaultV1Instances(path string, num int) Instances {
	ins := make([]*Instance, num)
	ins[0] = NewDefaultInstance(path)
	ins[0].SetName("etcd0")
	for i := 1; i < num; i++ {
		ins[i] = NewDefaultInstance(path)
		ins[i].SetName(fmt.Sprintf("etcd%d", i))
		ins[i].SetV1PeerAddr(fmt.Sprintf("127.0.0.1:%d", 7001+i))
		ins[i].SetV1Addr(fmt.Sprintf("127.0.0.1:%d", 4001+i))
		ins[i].SetV1Peers([]string{"127.0.0.1:7001"})
	}
	return ins
}

func NewDefaultV1InstancesThroughDiscovery(path string, num int, url string) Instances {
	ins := make([]*Instance, num)
	for i := range ins {
		ins[i] = NewDefaultInstance(path)
		ins[i].SetName(fmt.Sprintf("etcd%d", i))
		ins[i].SetDiscovery(url)
		ins[i].SetV1PeerAddr(fmt.Sprintf("127.0.0.1:%d", 7001+i))
		ins[i].SetV1Addr(fmt.Sprintf("127.0.0.1:%d", 4001+i))
	}
	return ins

}

func (ins Instances) Clone(path string) Instances {
	nins := make([]*Instance, len(ins))
	for i := range ins {
		nins[i] = ins[i].Clone(path)
	}
	return nins
}

func (ins Instances) CleanUnsuppportedV1Flags() {
	for _, i := range ins {
		i.CleanUnsuppportedV1Flags()
	}
}

func (ins Instances) Start() error {
	for _, i := range ins {
		if err := i.Start(); err != nil {
			return err
		}
	}
	// leave time for instances to sync and write some entries into disk
	// TODO: use more reliable method
	time.Sleep(time.Second)
	return nil
}

func (ins Instances) Wait() error {
	for _, i := range ins {
		if err := i.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func (ins Instances) Stop() {
	for _, i := range ins {
		i.Stop()
	}
}

func (ins Instances) Terminate() {
	for _, i := range ins {
		i.Terminate()
	}
}
