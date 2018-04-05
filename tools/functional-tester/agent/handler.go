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
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/proxy"
	"github.com/coreos/etcd/tools/functional-tester/rpcpb"

	"go.uber.org/zap"
)

// return error for system errors (e.g. fail to create files)
// return status error in response for wrong configuration/operation (e.g. start etcd twice)
func (srv *Server) handleTesterRequest(req *rpcpb.Request) (resp *rpcpb.Response, err error) {
	defer func() {
		if err == nil {
			srv.last = req.Operation
			srv.lg.Info("handler success", zap.String("operation", req.Operation.String()))
		}
	}()

	switch req.Operation {
	case rpcpb.Operation_InitialStartEtcd:
		return srv.handleInitialStartEtcd(req)
	case rpcpb.Operation_RestartEtcd:
		return srv.handleRestartEtcd()
	case rpcpb.Operation_KillEtcd:
		return srv.handleKillEtcd()
	case rpcpb.Operation_FailArchive:
		return srv.handleFailArchive()
	case rpcpb.Operation_DestroyEtcdAgent:
		return srv.handleDestroyEtcdAgent()

	case rpcpb.Operation_BlackholePeerPortTxRx:
		return srv.handleBlackholePeerPortTxRx()
	case rpcpb.Operation_UnblackholePeerPortTxRx:
		return srv.handleUnblackholePeerPortTxRx()
	case rpcpb.Operation_DelayPeerPortTxRx:
		return srv.handleDelayPeerPortTxRx()
	case rpcpb.Operation_UndelayPeerPortTxRx:
		return srv.handleUndelayPeerPortTxRx()

	default:
		msg := fmt.Sprintf("operation not found (%v)", req.Operation)
		return &rpcpb.Response{Success: false, Status: msg}, errors.New(msg)
	}
}

func (srv *Server) handleInitialStartEtcd(req *rpcpb.Request) (*rpcpb.Response, error) {
	if srv.last != rpcpb.Operation_NotStarted {
		return &rpcpb.Response{
			Success: false,
			Status:  fmt.Sprintf("%q is not valid; last server operation was %q", rpcpb.Operation_InitialStartEtcd.String(), srv.last.String()),
			Member:  req.Member,
		}, nil
	}

	srv.Member = req.Member
	srv.Tester = req.Tester

	err := fileutil.TouchDirAll(srv.Member.BaseDir)
	if err != nil {
		return nil, err
	}
	srv.lg.Info("created base directory", zap.String("path", srv.Member.BaseDir))

	if err = srv.saveEtcdLogFile(); err != nil {
		return nil, err
	}

	srv.creatEtcdCmd()

	if err = srv.saveTLSAssets(); err != nil {
		return nil, err
	}
	if err = srv.startEtcdCmd(); err != nil {
		return nil, err
	}
	srv.lg.Info("started etcd", zap.String("command-path", srv.etcdCmd.Path))
	if err = srv.loadAutoTLSAssets(); err != nil {
		return nil, err
	}

	// wait some time for etcd listener start
	// before setting up proxy
	time.Sleep(time.Second)
	if err = srv.startProxy(); err != nil {
		return nil, err
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "start etcd PASS",
		Member:  srv.Member,
	}, nil
}

func (srv *Server) startProxy() error {
	if srv.Member.EtcdClientProxy {
		advertiseClientURL, advertiseClientURLPort, err := getURLAndPort(srv.Member.Etcd.AdvertiseClientURLs[0])
		if err != nil {
			return err
		}
		listenClientURL, _, err := getURLAndPort(srv.Member.Etcd.ListenClientURLs[0])
		if err != nil {
			return err
		}

		srv.advertiseClientPortToProxy[advertiseClientURLPort] = proxy.NewServer(proxy.ServerConfig{
			Logger: srv.lg,
			From:   *advertiseClientURL,
			To:     *listenClientURL,
		})
		select {
		case err = <-srv.advertiseClientPortToProxy[advertiseClientURLPort].Error():
			return err
		case <-time.After(2 * time.Second):
			srv.lg.Info("started proxy on client traffic", zap.String("url", advertiseClientURL.String()))
		}
	}

	if srv.Member.EtcdPeerProxy {
		advertisePeerURL, advertisePeerURLPort, err := getURLAndPort(srv.Member.Etcd.AdvertisePeerURLs[0])
		if err != nil {
			return err
		}
		listenPeerURL, _, err := getURLAndPort(srv.Member.Etcd.ListenPeerURLs[0])
		if err != nil {
			return err
		}

		srv.advertisePeerPortToProxy[advertisePeerURLPort] = proxy.NewServer(proxy.ServerConfig{
			Logger: srv.lg,
			From:   *advertisePeerURL,
			To:     *listenPeerURL,
		})
		select {
		case err = <-srv.advertisePeerPortToProxy[advertisePeerURLPort].Error():
			return err
		case <-time.After(2 * time.Second):
			srv.lg.Info("started proxy on peer traffic", zap.String("url", advertisePeerURL.String()))
		}
	}
	return nil
}

func (srv *Server) stopProxy() {
	if srv.Member.EtcdClientProxy && len(srv.advertiseClientPortToProxy) > 0 {
		for port, px := range srv.advertiseClientPortToProxy {
			if err := px.Close(); err != nil {
				srv.lg.Warn("failed to close proxy", zap.Int("port", port))
				continue
			}
			select {
			case <-px.Done():
				// enough time to release port
				time.Sleep(time.Second)
			case <-time.After(time.Second):
			}
			srv.lg.Info("closed proxy",
				zap.Int("port", port),
				zap.String("from", px.From()),
				zap.String("to", px.To()),
			)
		}
		srv.advertiseClientPortToProxy = make(map[int]proxy.Server)
	}
	if srv.Member.EtcdPeerProxy && len(srv.advertisePeerPortToProxy) > 0 {
		for port, px := range srv.advertisePeerPortToProxy {
			if err := px.Close(); err != nil {
				srv.lg.Warn("failed to close proxy", zap.Int("port", port))
				continue
			}
			select {
			case <-px.Done():
				// enough time to release port
				time.Sleep(time.Second)
			case <-time.After(time.Second):
			}
			srv.lg.Info("closed proxy",
				zap.Int("port", port),
				zap.String("from", px.From()),
				zap.String("to", px.To()),
			)
		}
		srv.advertisePeerPortToProxy = make(map[int]proxy.Server)
	}
}

func (srv *Server) saveEtcdLogFile() error {
	var err error
	srv.etcdLogFile, err = os.Create(srv.Member.EtcdLogPath)
	if err != nil {
		return err
	}
	srv.lg.Info("created etcd log file", zap.String("path", srv.Member.EtcdLogPath))
	return nil
}

func (srv *Server) creatEtcdCmd() {
	etcdPath, etcdFlags := srv.Member.EtcdExecPath, srv.Member.Etcd.Flags()
	u, _ := url.Parse(srv.Member.FailpointHTTPAddr)
	srv.lg.Info("creating etcd command",
		zap.String("etcd-exec-path", etcdPath),
		zap.Strings("etcd-flags", etcdFlags),
		zap.String("failpoint-http-addr", srv.Member.FailpointHTTPAddr),
		zap.String("failpoint-addr", u.Host),
	)
	srv.etcdCmd = exec.Command(etcdPath, etcdFlags...)
	srv.etcdCmd.Env = []string{"GOFAIL_HTTP=" + u.Host}
	srv.etcdCmd.Stdout = srv.etcdLogFile
	srv.etcdCmd.Stderr = srv.etcdLogFile
}

func (srv *Server) saveTLSAssets() error {
	// if started with manual TLS, stores TLS assets
	// from tester/client to disk before starting etcd process
	// TODO: not implemented yet
	if !srv.Member.Etcd.ClientAutoTLS {
		if srv.Member.Etcd.ClientCertAuth {
			return fmt.Errorf("manual TLS setup is not implemented yet, but Member.Etcd.ClientCertAuth is %v", srv.Member.Etcd.ClientCertAuth)
		}
		if srv.Member.Etcd.ClientCertFile != "" {
			return fmt.Errorf("manual TLS setup is not implemented yet, but Member.Etcd.ClientCertFile is %q", srv.Member.Etcd.ClientCertFile)
		}
		if srv.Member.Etcd.ClientKeyFile != "" {
			return fmt.Errorf("manual TLS setup is not implemented yet, but Member.Etcd.ClientKeyFile is %q", srv.Member.Etcd.ClientKeyFile)
		}
		if srv.Member.Etcd.ClientTrustedCAFile != "" {
			return fmt.Errorf("manual TLS setup is not implemented yet, but Member.Etcd.ClientTrustedCAFile is %q", srv.Member.Etcd.ClientTrustedCAFile)
		}
	}
	if !srv.Member.Etcd.PeerAutoTLS {
		if srv.Member.Etcd.PeerClientCertAuth {
			return fmt.Errorf("manual TLS setup is not implemented yet, but Member.Etcd.PeerClientCertAuth is %v", srv.Member.Etcd.PeerClientCertAuth)
		}
		if srv.Member.Etcd.PeerCertFile != "" {
			return fmt.Errorf("manual TLS setup is not implemented yet, but Member.Etcd.PeerCertFile is %q", srv.Member.Etcd.PeerCertFile)
		}
		if srv.Member.Etcd.PeerKeyFile != "" {
			return fmt.Errorf("manual TLS setup is not implemented yet, but Member.Etcd.PeerKeyFile is %q", srv.Member.Etcd.PeerKeyFile)
		}
		if srv.Member.Etcd.PeerTrustedCAFile != "" {
			return fmt.Errorf("manual TLS setup is not implemented yet, but Member.Etcd.PeerTrustedCAFile is %q", srv.Member.Etcd.PeerTrustedCAFile)
		}
	}

	// TODO
	return nil
}

func (srv *Server) loadAutoTLSAssets() error {
	// if started with auto TLS, sends back TLS assets to tester/client
	if srv.Member.Etcd.ClientAutoTLS {
		fdir := filepath.Join(srv.Member.Etcd.DataDir, "fixtures", "client")

		certPath := filepath.Join(fdir, "cert.pem")
		if !fileutil.Exist(certPath) {
			return fmt.Errorf("cannot find %q", certPath)
		}
		certData, err := ioutil.ReadFile(certPath)
		if err != nil {
			return fmt.Errorf("cannot read %q (%v)", certPath, err)
		}
		srv.Member.ClientCertData = string(certData)

		keyPath := filepath.Join(fdir, "key.pem")
		if !fileutil.Exist(keyPath) {
			return fmt.Errorf("cannot find %q", keyPath)
		}
		keyData, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("cannot read %q (%v)", keyPath, err)
		}
		srv.Member.ClientKeyData = string(keyData)

		srv.lg.Info(
			"loaded client TLS assets",
			zap.String("peer-cert-path", certPath),
			zap.Int("peer-cert-length", len(certData)),
			zap.String("peer-key-path", keyPath),
			zap.Int("peer-key-length", len(keyData)),
		)
	}
	if srv.Member.Etcd.ClientAutoTLS {
		fdir := filepath.Join(srv.Member.Etcd.DataDir, "fixtures", "peer")

		certPath := filepath.Join(fdir, "cert.pem")
		if !fileutil.Exist(certPath) {
			return fmt.Errorf("cannot find %q", certPath)
		}
		certData, err := ioutil.ReadFile(certPath)
		if err != nil {
			return fmt.Errorf("cannot read %q (%v)", certPath, err)
		}
		srv.Member.PeerCertData = string(certData)

		keyPath := filepath.Join(fdir, "key.pem")
		if !fileutil.Exist(keyPath) {
			return fmt.Errorf("cannot find %q", keyPath)
		}
		keyData, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("cannot read %q (%v)", keyPath, err)
		}
		srv.Member.PeerKeyData = string(keyData)

		srv.lg.Info(
			"loaded peer TLS assets",
			zap.String("peer-cert-path", certPath),
			zap.Int("peer-cert-length", len(certData)),
			zap.String("peer-key-path", keyPath),
			zap.Int("peer-key-length", len(keyData)),
		)
	}
	return nil
}

// start but do not wait for it to complete
func (srv *Server) startEtcdCmd() error {
	return srv.etcdCmd.Start()
}

func (srv *Server) handleRestartEtcd() (*rpcpb.Response, error) {
	srv.creatEtcdCmd()

	var err error
	if err = srv.saveTLSAssets(); err != nil {
		return nil, err
	}
	if err = srv.startEtcdCmd(); err != nil {
		return nil, err
	}
	srv.lg.Info("restarted etcd", zap.String("command-path", srv.etcdCmd.Path))
	if err = srv.loadAutoTLSAssets(); err != nil {
		return nil, err
	}

	// wait some time for etcd listener start
	// before setting up proxy
	// TODO: local tests should handle port conflicts
	// with clients on restart
	time.Sleep(time.Second)
	if err = srv.startProxy(); err != nil {
		return nil, err
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "restart etcd PASS",
		Member:  srv.Member,
	}, nil
}

func (srv *Server) handleKillEtcd() (*rpcpb.Response, error) {
	srv.stopProxy()

	err := stopWithSig(srv.etcdCmd, syscall.SIGTERM)
	if err != nil {
		return nil, err
	}
	srv.lg.Info("killed etcd", zap.String("signal", syscall.SIGTERM.String()))

	return &rpcpb.Response{
		Success: true,
		Status:  "successfully killed etcd!",
	}, nil
}

func (srv *Server) handleFailArchive() (*rpcpb.Response, error) {
	srv.stopProxy()

	// exit with stackstrace
	err := stopWithSig(srv.etcdCmd, syscall.SIGQUIT)
	if err != nil {
		return nil, err
	}
	srv.lg.Info("killed etcd", zap.String("signal", syscall.SIGQUIT.String()))

	srv.etcdLogFile.Sync()
	srv.etcdLogFile.Close()

	// TODO: support separate WAL directory
	if err = archive(
		srv.Member.BaseDir,
		srv.Member.EtcdLogPath,
		srv.Member.Etcd.DataDir,
	); err != nil {
		return nil, err
	}
	srv.lg.Info("archived data", zap.String("base-dir", srv.Member.BaseDir))

	if err = srv.saveEtcdLogFile(); err != nil {
		return nil, err
	}

	srv.lg.Info("cleaning up page cache")
	if err := cleanPageCache(); err != nil {
		srv.lg.Warn("failed to clean up page cache", zap.String("error", err.Error()))
	}
	srv.lg.Info("cleaned up page cache")

	return &rpcpb.Response{
		Success: true,
		Status:  "successfully cleaned up etcd!",
	}, nil
}

// stop proxy, etcd, delete data directory
func (srv *Server) handleDestroyEtcdAgent() (*rpcpb.Response, error) {
	err := stopWithSig(srv.etcdCmd, syscall.SIGTERM)
	if err != nil {
		return nil, err
	}
	srv.lg.Info("killed etcd", zap.String("signal", syscall.SIGTERM.String()))

	err = os.RemoveAll(srv.Member.BaseDir)
	if err != nil {
		return nil, err
	}
	srv.lg.Info("removed base directory", zap.String("dir", srv.Member.BaseDir))

	// stop agent server
	srv.Stop()

	for port, px := range srv.advertiseClientPortToProxy {
		err := px.Close()
		srv.lg.Info("closed proxy", zap.Int("client-port", port), zap.Error(err))
	}
	for port, px := range srv.advertisePeerPortToProxy {
		err := px.Close()
		srv.lg.Info("closed proxy", zap.Int("peer-port", port), zap.Error(err))
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "successfully destroyed etcd and agent!",
	}, nil
}

func (srv *Server) handleBlackholePeerPortTxRx() (*rpcpb.Response, error) {
	for port, px := range srv.advertisePeerPortToProxy {
		srv.lg.Info("blackholing", zap.Int("peer-port", port))
		px.BlackholeTx()
		px.BlackholeRx()
		srv.lg.Info("blackholed", zap.Int("peer-port", port))
	}
	return &rpcpb.Response{
		Success: true,
		Status:  "successfully blackholed peer port tx/rx!",
	}, nil
}

func (srv *Server) handleUnblackholePeerPortTxRx() (*rpcpb.Response, error) {
	for port, px := range srv.advertisePeerPortToProxy {
		srv.lg.Info("unblackholing", zap.Int("peer-port", port))
		px.UnblackholeTx()
		px.UnblackholeRx()
		srv.lg.Info("unblackholed", zap.Int("peer-port", port))
	}
	return &rpcpb.Response{
		Success: true,
		Status:  "successfully unblackholed peer port tx/rx!",
	}, nil
}

func (srv *Server) handleDelayPeerPortTxRx() (*rpcpb.Response, error) {
	lat := time.Duration(srv.Tester.UpdatedDelayLatencyMs) * time.Millisecond
	rv := time.Duration(srv.Tester.DelayLatencyMsRv) * time.Millisecond

	for port, px := range srv.advertisePeerPortToProxy {
		srv.lg.Info("delaying",
			zap.Int("peer-port", port),
			zap.Duration("latency", lat),
			zap.Duration("random-variable", rv),
		)
		px.DelayTx(lat, rv)
		px.DelayRx(lat, rv)
		srv.lg.Info("delayed",
			zap.Int("peer-port", port),
			zap.Duration("latency", lat),
			zap.Duration("random-variable", rv),
		)
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "successfully delay peer port tx/rx!",
	}, nil
}

func (srv *Server) handleUndelayPeerPortTxRx() (*rpcpb.Response, error) {
	for port, px := range srv.advertisePeerPortToProxy {
		srv.lg.Info("undelaying", zap.Int("peer-port", port))
		px.UndelayTx()
		px.UndelayRx()
		srv.lg.Info("undelayed", zap.Int("peer-port", port))
	}
	return &rpcpb.Response{
		Success: true,
		Status:  "successfully undelay peer port tx/rx!",
	}, nil
}
