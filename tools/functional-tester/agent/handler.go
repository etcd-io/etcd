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
	"net/url"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/tools/functional-tester/rpcpb"

	"go.uber.org/zap"
)

// return error for system errors (e.g. fail to create files)
// return status error in response for wrong configuration/operation (e.g. start etcd twice)
func (srv *Server) handleTesterRequest(req *rpcpb.Request) (resp *rpcpb.Response, err error) {
	defer func() {
		if err == nil {
			srv.last = req.Operation
			srv.logger.Info("handler success", zap.String("operation", req.Operation.String()))
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
		}, nil
	}

	srv.Member = req.Member
	srv.Tester = req.Tester

	srv.logger.Info("creating base directory", zap.String("path", srv.Member.BaseDir))
	err := fileutil.TouchDirAll(srv.Member.BaseDir)
	if err != nil {
		return nil, err
	}
	srv.logger.Info("created base directory", zap.String("path", srv.Member.BaseDir))

	if err = srv.createEtcdFile(); err != nil {
		return nil, err
	}
	srv.creatEtcdCmd()

	srv.logger.Info("starting etcd")
	err = srv.startEtcdCmd()
	if err != nil {
		return nil, err
	}
	srv.logger.Info("started etcd", zap.String("command-path", srv.etcdCmd.Path))

	// wait some time for etcd listener start
	// before setting up proxy
	time.Sleep(time.Second)
	if err = srv.startProxy(); err != nil {
		return nil, err
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "successfully started etcd!",
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

		srv.logger.Info("starting proxy on client traffic", zap.String("url", advertiseClientURL.String()))
		srv.advertiseClientPortToProxy[advertiseClientURLPort] = transport.NewProxy(transport.ProxyConfig{
			Logger: srv.logger,
			From:   *advertiseClientURL,
			To:     *listenClientURL,
		})
		select {
		case err = <-srv.advertiseClientPortToProxy[advertiseClientURLPort].Error():
			return err
		case <-time.After(2 * time.Second):
			srv.logger.Info("started proxy on client traffic", zap.String("url", advertiseClientURL.String()))
		}
	}

	if srv.Member.EtcdPeerProxy {
		advertisePeerURL, advertisePeerURLPort, err := getURLAndPort(srv.Member.Etcd.InitialAdvertisePeerURLs[0])
		if err != nil {
			return err
		}
		listenPeerURL, _, err := getURLAndPort(srv.Member.Etcd.ListenPeerURLs[0])
		if err != nil {
			return err
		}

		srv.logger.Info("starting proxy on peer traffic", zap.String("url", advertisePeerURL.String()))
		srv.advertisePeerPortToProxy[advertisePeerURLPort] = transport.NewProxy(transport.ProxyConfig{
			Logger: srv.logger,
			From:   *advertisePeerURL,
			To:     *listenPeerURL,
		})
		select {
		case err = <-srv.advertisePeerPortToProxy[advertisePeerURLPort].Error():
			return err
		case <-time.After(2 * time.Second):
			srv.logger.Info("started proxy on peer traffic", zap.String("url", advertisePeerURL.String()))
		}
	}
	return nil
}

func (srv *Server) stopProxy() {
	if srv.Member.EtcdClientProxy && len(srv.advertiseClientPortToProxy) > 0 {
		for port, px := range srv.advertiseClientPortToProxy {
			srv.logger.Info("closing proxy",
				zap.Int("port", port),
				zap.String("from", px.From()),
				zap.String("to", px.To()),
			)
			if err := px.Close(); err != nil {
				srv.logger.Warn("failed to close proxy", zap.Int("port", port))
				continue
			}
			select {
			case <-px.Done():
				// enough time to release port
				time.Sleep(time.Second)
			case <-time.After(time.Second):
			}
			srv.logger.Info("closed proxy",
				zap.Int("port", port),
				zap.String("from", px.From()),
				zap.String("to", px.To()),
			)
		}
		srv.advertiseClientPortToProxy = make(map[int]transport.Proxy)
	}
	if srv.Member.EtcdPeerProxy && len(srv.advertisePeerPortToProxy) > 0 {
		for port, px := range srv.advertisePeerPortToProxy {
			srv.logger.Info("closing proxy",
				zap.Int("port", port),
				zap.String("from", px.From()),
				zap.String("to", px.To()),
			)
			if err := px.Close(); err != nil {
				srv.logger.Warn("failed to close proxy", zap.Int("port", port))
				continue
			}
			select {
			case <-px.Done():
				// enough time to release port
				time.Sleep(time.Second)
			case <-time.After(time.Second):
			}
			srv.logger.Info("closed proxy",
				zap.Int("port", port),
				zap.String("from", px.From()),
				zap.String("to", px.To()),
			)
		}
		srv.advertisePeerPortToProxy = make(map[int]transport.Proxy)
	}
}

func (srv *Server) createEtcdFile() error {
	srv.logger.Info("creating etcd log file", zap.String("path", srv.Member.EtcdLogPath))
	var err error
	srv.etcdLogFile, err = os.Create(srv.Member.EtcdLogPath)
	if err != nil {
		return err
	}
	srv.logger.Info("created etcd log file", zap.String("path", srv.Member.EtcdLogPath))
	return nil
}

func (srv *Server) creatEtcdCmd() {
	etcdPath, etcdFlags := srv.Member.EtcdExecPath, srv.Member.Etcd.Flags()
	u, _ := url.Parse(srv.Member.FailpointHTTPAddr)
	srv.logger.Info("creating etcd command",
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

// start but do not wait for it to complete
func (srv *Server) startEtcdCmd() error {
	return srv.etcdCmd.Start()
}

func (srv *Server) handleRestartEtcd() (*rpcpb.Response, error) {
	srv.creatEtcdCmd()

	srv.logger.Info("restarting etcd")
	err := srv.startEtcdCmd()
	if err != nil {
		return nil, err
	}
	srv.logger.Info("restarted etcd", zap.String("command-path", srv.etcdCmd.Path))

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
		Status:  "successfully restarted etcd!",
	}, nil
}

func (srv *Server) handleKillEtcd() (*rpcpb.Response, error) {
	srv.stopProxy()

	srv.logger.Info("killing etcd", zap.String("signal", syscall.SIGTERM.String()))
	err := stopWithSig(srv.etcdCmd, syscall.SIGTERM)
	if err != nil {
		return nil, err
	}
	srv.logger.Info("killed etcd", zap.String("signal", syscall.SIGTERM.String()))

	return &rpcpb.Response{
		Success: true,
		Status:  "successfully killed etcd!",
	}, nil
}

func (srv *Server) handleFailArchive() (*rpcpb.Response, error) {
	srv.stopProxy()

	// exit with stackstrace
	srv.logger.Info("killing etcd", zap.String("signal", syscall.SIGQUIT.String()))
	err := stopWithSig(srv.etcdCmd, syscall.SIGQUIT)
	if err != nil {
		return nil, err
	}
	srv.logger.Info("killed etcd", zap.String("signal", syscall.SIGQUIT.String()))

	srv.etcdLogFile.Sync()
	srv.etcdLogFile.Close()

	// TODO: support separate WAL directory
	srv.logger.Info("archiving data", zap.String("base-dir", srv.Member.BaseDir))
	if err = archive(
		srv.Member.BaseDir,
		srv.Member.EtcdLogPath,
		srv.Member.Etcd.DataDir,
	); err != nil {
		return nil, err
	}
	srv.logger.Info("archived data", zap.String("base-dir", srv.Member.BaseDir))

	if err = srv.createEtcdFile(); err != nil {
		return nil, err
	}

	srv.logger.Info("cleaning up page cache")
	if err := cleanPageCache(); err != nil {
		srv.logger.Warn("failed to clean up page cache", zap.String("error", err.Error()))
	}
	srv.logger.Info("cleaned up page cache")

	return &rpcpb.Response{
		Success: true,
		Status:  "successfully cleaned up etcd!",
	}, nil
}

// stop proxy, etcd, delete data directory
func (srv *Server) handleDestroyEtcdAgent() (*rpcpb.Response, error) {
	srv.logger.Info("killing etcd", zap.String("signal", syscall.SIGTERM.String()))
	err := stopWithSig(srv.etcdCmd, syscall.SIGTERM)
	if err != nil {
		return nil, err
	}
	srv.logger.Info("killed etcd", zap.String("signal", syscall.SIGTERM.String()))

	srv.logger.Info("removing base directory", zap.String("dir", srv.Member.BaseDir))
	err = os.RemoveAll(srv.Member.BaseDir)
	if err != nil {
		return nil, err
	}
	srv.logger.Info("removed base directory", zap.String("dir", srv.Member.BaseDir))

	// stop agent server
	srv.Stop()

	for port, px := range srv.advertiseClientPortToProxy {
		srv.logger.Info("closing proxy", zap.Int("client-port", port))
		err := px.Close()
		srv.logger.Info("closed proxy", zap.Int("client-port", port), zap.Error(err))
	}
	for port, px := range srv.advertisePeerPortToProxy {
		srv.logger.Info("closing proxy", zap.Int("peer-port", port))
		err := px.Close()
		srv.logger.Info("closed proxy", zap.Int("peer-port", port), zap.Error(err))
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "successfully destroyed etcd and agent!",
	}, nil
}

func (srv *Server) handleBlackholePeerPortTxRx() (*rpcpb.Response, error) {
	for port, px := range srv.advertisePeerPortToProxy {
		srv.logger.Info("blackholing", zap.Int("peer-port", port))
		px.BlackholeTx()
		px.BlackholeRx()
		srv.logger.Info("blackholed", zap.Int("peer-port", port))
	}
	return &rpcpb.Response{
		Success: true,
		Status:  "successfully blackholed peer port tx/rx!",
	}, nil
}

func (srv *Server) handleUnblackholePeerPortTxRx() (*rpcpb.Response, error) {
	for port, px := range srv.advertisePeerPortToProxy {
		srv.logger.Info("unblackholing", zap.Int("peer-port", port))
		px.UnblackholeTx()
		px.UnblackholeRx()
		srv.logger.Info("unblackholed", zap.Int("peer-port", port))
	}
	return &rpcpb.Response{
		Success: true,
		Status:  "successfully unblackholed peer port tx/rx!",
	}, nil
}

func (srv *Server) handleDelayPeerPortTxRx() (*rpcpb.Response, error) {
	lat := time.Duration(srv.Tester.DelayLatencyMs) * time.Millisecond
	rv := time.Duration(srv.Tester.DelayLatencyMsRv) * time.Millisecond

	for port, px := range srv.advertisePeerPortToProxy {
		srv.logger.Info("delaying",
			zap.Int("peer-port", port),
			zap.Duration("latency", lat),
			zap.Duration("random-variable", rv),
		)
		px.DelayTx(lat, rv)
		px.DelayRx(lat, rv)
		srv.logger.Info("delayed",
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
		srv.logger.Info("undelaying", zap.Int("peer-port", port))
		px.UndelayTx()
		px.UndelayRx()
		srv.logger.Info("undelayed", zap.Int("peer-port", port))
	}
	return &rpcpb.Response{
		Success: true,
		Status:  "successfully undelay peer port tx/rx!",
	}, nil
}
