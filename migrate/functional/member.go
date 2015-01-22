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

type Proc struct {
	*exec.Cmd
	Name    string
	DataDir string
	URL     string
	PeerURL string

	stderr io.ReadCloser
}

func NewProcWithDefaultFlags(path string) *Proc {
	var args []string
	dir, err := ioutil.TempDir(os.TempDir(), "etcd")
	if err != nil {
		fmt.Printf("unexpected TempDir error: %v", err)
		os.Exit(1)
	}
	args = append(args, "--data-dir="+dir)
	args = append(args, "--name=default")
	p := &Proc{
		Cmd:     exec.Command(path, args...),
		Name:    "default",
		DataDir: dir,
		URL:     "http://127.0.0.1:4001",
		PeerURL: "http://127.0.0.1:7001",
	}
	// always expect to use start_desired_verson mode
	p.Env = append(p.Env,
		"ETCD_ALLOW_LEGACY_MODE=true",
		"ETCD_BINARY_DIR="+binDir,
	)
	return p
}

func NewProcWithV1Flags(path string) *Proc {
	p := NewProcWithDefaultFlags(path)
	p.SetV1PeerAddr("127.0.0.1:7001")
	return p
}

func NewProcWithV2Flags(path string) *Proc {
	p := NewProcWithDefaultFlags(path)
	p.SetV2PeerURL("http://127.0.0.1:7001")
	return p
}

func (p *Proc) SetV2PeerURL(url string) {
	p.Args = append(p.Args,
		"-listen-peer-urls="+url,
		"-initial-advertise-peer-urls="+url,
		"-initial-cluster",
		p.Name+"="+url,
	)
	p.PeerURL = url
}

func (p *Proc) SetV1PeerAddr(addr string) {
	p.Args = append(p.Args,
		"-peer-addr="+addr,
	)
	p.PeerURL = "http://" + addr
}

func (p *Proc) SetV1Addr(addr string) {
	p.Args = append(p.Args,
		"-addr="+addr,
	)
	p.URL = "http://" + addr
}

func (p *Proc) SetV1Peers(peers []string) {
	p.Args = append(p.Args,
		"-peers="+strings.Join(peers, ","),
	)
}

func (p *Proc) SetName(name string) {
	p.Args = append(p.Args,
		"-name="+name,
	)
	p.Name = name
}

func (p *Proc) SetDataDir(dataDir string) {
	p.Args = append(p.Args,
		"-data-dir="+dataDir,
	)
	p.DataDir = dataDir
}

func (p *Proc) SetSnapCount(cnt int) {
	p.Args = append(p.Args,
		"-snapshot-count="+strconv.Itoa(cnt),
	)
}

func (p *Proc) SetDiscovery(url string) {
	p.Args = append(p.Args,
		"-discovery="+url,
	)
}

func (p *Proc) CleanUnsuppportedV1Flags() {
	var args []string
	for _, arg := range p.Args {
		if !strings.HasPrefix(arg, "-peers=") {
			args = append(args, arg)
		}
	}
	p.Args = args
}

func (p *Proc) Start() error {
	var err error
	if p.stderr, err = p.Cmd.StderrPipe(); err != nil {
		return err
	}
	if err := p.Cmd.Start(); err != nil {
		return err
	}
	for k := 0; k < 50; k++ {
		_, err := http.Get(p.URL)
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	errMsg, _ := ioutil.ReadAll(p.stderr)
	return fmt.Errorf("instance %s failed to be available after a long time: %s", p.Name, errMsg)
}

func (p *Proc) Stop() {
	if err := p.Cmd.Process.Kill(); err != nil {
		fmt.Printf("Process Kill error: %v", err)
		return
	}
	ioutil.ReadAll(p.stderr)
	p.Cmd.Wait()
}

func (p *Proc) Restart() error {
	p.Stop()
	return p.Start()
}

func (p *Proc) Terminate() {
	p.Stop()
	os.RemoveAll(p.DataDir)
}

type ProcGroup []*Proc

func NewProcGroupWithV1Flags(path string, num int) ProcGroup {
	pg := make([]*Proc, num)
	pg[0] = NewProcWithDefaultFlags(path)
	pg[0].SetName("etcd0")
	for i := 1; i < num; i++ {
		pg[i] = NewProcWithDefaultFlags(path)
		pg[i].SetName(fmt.Sprintf("etcd%d", i))
		pg[i].SetV1PeerAddr(fmt.Sprintf("127.0.0.1:%d", 7001+i))
		pg[i].SetV1Addr(fmt.Sprintf("127.0.0.1:%d", 4001+i))
		pg[i].SetV1Peers([]string{"127.0.0.1:7001"})
	}
	return pg
}

func NewProcGroupViaDiscoveryWithV1Flags(path string, num int, url string) ProcGroup {
	pg := make([]*Proc, num)
	for i := range pg {
		pg[i] = NewProcWithDefaultFlags(path)
		pg[i].SetName(fmt.Sprintf("etcd%d", i))
		pg[i].SetDiscovery(url)
		pg[i].SetV1PeerAddr(fmt.Sprintf("127.0.0.1:%d", 7001+i))
		pg[i].SetV1Addr(fmt.Sprintf("127.0.0.1:%d", 4001+i))
	}
	return pg
}

func (pg ProcGroup) InheritDataDir(opg ProcGroup) {
	for i := range pg {
		pg[i].SetDataDir(opg[i].DataDir)
	}
}

func (pg ProcGroup) SetSnapCount(count int) {
	for i := range pg {
		pg[i].SetSnapCount(count)
	}
}

func (pg ProcGroup) CleanUnsuppportedV1Flags() {
	for _, p := range pg {
		p.CleanUnsuppportedV1Flags()
	}
}

func (pg ProcGroup) Start() error {
	for _, p := range pg {
		if err := p.Start(); err != nil {
			return err
		}
	}
	// leave time for instances to sync and write some entries into disk
	// TODO: use more reliable method
	time.Sleep(time.Second)
	return nil
}

func (pg ProcGroup) Wait() error {
	for _, p := range pg {
		if err := p.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func (pg ProcGroup) Stop() {
	for _, p := range pg {
		p.Stop()
	}
}

func (pg ProcGroup) Terminate() {
	for _, p := range pg {
		p.Terminate()
	}
}
