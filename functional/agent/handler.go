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

	"go.etcd.io/etcd/v3/functional/rpcpb"
	"go.etcd.io/etcd/v3/pkg/fileutil"
	"go.etcd.io/etcd/v3/pkg/proxy"

	"go.uber.org/zap"
)

// return error for system errors (e.g. fail to create files)
// return status error in response for wrong configuration/operation (e.g. start etcd twice)
func (srv *Server) handleTesterRequest(req *rpcpb.Request) (resp *rpcpb.Response, err error) {
	defer func() {
		if err == nil && req != nil {
			srv.last = req.Operation
			srv.lg.Info("handler success", zap.String("operation", req.Operation.String()))
		}
	}()
	if req != nil {
		srv.Member = req.Member
		srv.Tester = req.Tester
	}

	switch req.Operation {
	case rpcpb.Operation_INITIAL_START_ETCD:
		return srv.handle_INITIAL_START_ETCD(req)
	case rpcpb.Operation_RESTART_ETCD:
		return srv.handle_RESTART_ETCD(req)

	case rpcpb.Operation_SIGTERM_ETCD:
		return srv.handle_SIGTERM_ETCD()
	case rpcpb.Operation_SIGQUIT_ETCD_AND_REMOVE_DATA:
		return srv.handle_SIGQUIT_ETCD_AND_REMOVE_DATA()

	case rpcpb.Operation_SAVE_SNAPSHOT:
		return srv.handle_SAVE_SNAPSHOT()
	case rpcpb.Operation_RESTORE_RESTART_FROM_SNAPSHOT:
		return srv.handle_RESTORE_RESTART_FROM_SNAPSHOT(req)
	case rpcpb.Operation_RESTART_FROM_SNAPSHOT:
		return srv.handle_RESTART_FROM_SNAPSHOT(req)

	case rpcpb.Operation_SIGQUIT_ETCD_AND_ARCHIVE_DATA:
		return srv.handle_SIGQUIT_ETCD_AND_ARCHIVE_DATA()
	case rpcpb.Operation_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT:
		return srv.handle_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT()

	case rpcpb.Operation_BLACKHOLE_PEER_PORT_TX_RX:
		return srv.handle_BLACKHOLE_PEER_PORT_TX_RX(), nil
	case rpcpb.Operation_UNBLACKHOLE_PEER_PORT_TX_RX:
		return srv.handle_UNBLACKHOLE_PEER_PORT_TX_RX(), nil
	case rpcpb.Operation_DELAY_PEER_PORT_TX_RX:
		return srv.handle_DELAY_PEER_PORT_TX_RX(), nil
	case rpcpb.Operation_UNDELAY_PEER_PORT_TX_RX:
		return srv.handle_UNDELAY_PEER_PORT_TX_RX(), nil

	default:
		msg := fmt.Sprintf("operation not found (%v)", req.Operation)
		return &rpcpb.Response{Success: false, Status: msg}, errors.New(msg)
	}
}

// just archive the first file
func (srv *Server) createEtcdLogFile() error {
	var err error
	srv.etcdLogFile, err = os.Create(srv.Member.Etcd.LogOutputs[0])
	if err != nil {
		return err
	}
	srv.lg.Info("created etcd log file", zap.String("path", srv.Member.Etcd.LogOutputs[0]))
	return nil
}

func (srv *Server) creatEtcd(fromSnapshot bool, failpoints string) error {
	if !fileutil.Exist(srv.Member.EtcdExec) {
		return fmt.Errorf("unknown etcd exec path %q does not exist", srv.Member.EtcdExec)
	}

	etcdPath, etcdFlags := srv.Member.EtcdExec, srv.Member.Etcd.Flags()
	if fromSnapshot {
		etcdFlags = srv.Member.EtcdOnSnapshotRestore.Flags()
	}
	u, _ := url.Parse(srv.Member.FailpointHTTPAddr)
	srv.lg.Info(
		"creating etcd command",
		zap.String("etcd-exec", etcdPath),
		zap.Strings("etcd-flags", etcdFlags),
		zap.String("GOFAIL_FAILPOINTS", failpoints),
		zap.String("failpoint-http-addr", srv.Member.FailpointHTTPAddr),
		zap.String("failpoint-addr", u.Host),
	)
	srv.etcdCmd = exec.Command(etcdPath, etcdFlags...)
	srv.etcdCmd.Env = []string{"GOFAIL_HTTP=" + u.Host}
	if failpoints != "" {
		srv.etcdCmd.Env = append(srv.etcdCmd.Env, "GOFAIL_FAILPOINTS="+failpoints)
	}
	srv.etcdCmd.Stdout = srv.etcdLogFile
	srv.etcdCmd.Stderr = srv.etcdLogFile
	return nil
}

// start but do not wait for it to complete
func (srv *Server) runEtcd() error {
	errc := make(chan error)
	go func() {
		time.Sleep(5 * time.Second)
		// server advertise client/peer listener had to start first
		// before setting up proxy listener
		errc <- srv.startProxy()
	}()

	if srv.etcdCmd != nil {
		srv.lg.Info(
			"starting etcd command",
			zap.String("command-path", srv.etcdCmd.Path),
		)
		err := srv.etcdCmd.Start()
		perr := <-errc
		srv.lg.Info(
			"started etcd command",
			zap.String("command-path", srv.etcdCmd.Path),
			zap.Errors("errors", []error{err, perr}),
		)
		if err != nil {
			return err
		}
		return perr
	}

	select {
	case <-srv.etcdServer.Server.ReadyNotify():
		srv.lg.Info("embedded etcd is ready")
	case <-time.After(time.Minute):
		srv.etcdServer.Close()
		return fmt.Errorf("took too long to start %v", <-srv.etcdServer.Err())
	}
	return <-errc
}

// SIGQUIT to exit with stackstrace
func (srv *Server) stopEtcd(sig os.Signal) error {
	srv.stopProxy()

	if srv.etcdCmd != nil {
		srv.lg.Info(
			"stopping etcd command",
			zap.String("command-path", srv.etcdCmd.Path),
			zap.String("signal", sig.String()),
		)

		err := srv.etcdCmd.Process.Signal(sig)
		if err != nil {
			return err
		}

		errc := make(chan error)
		go func() {
			_, ew := srv.etcdCmd.Process.Wait()
			errc <- ew
			close(errc)
		}()

		select {
		case <-time.After(5 * time.Second):
			srv.etcdCmd.Process.Kill()
		case e := <-errc:
			return e
		}

		err = <-errc

		srv.lg.Info(
			"stopped etcd command",
			zap.String("command-path", srv.etcdCmd.Path),
			zap.String("signal", sig.String()),
			zap.Error(err),
		)
		return err
	}

	srv.lg.Info("stopping embedded etcd")
	srv.etcdServer.Server.HardStop()
	srv.etcdServer.Close()
	srv.lg.Info("stopped embedded etcd")
	return nil
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

		srv.lg.Info("starting proxy on client traffic", zap.String("url", advertiseClientURL.String()))
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

		srv.lg.Info("starting proxy on peer traffic", zap.String("url", advertisePeerURL.String()))
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

// if started with manual TLS, stores TLS assets
// from tester/client to disk before starting etcd process
func (srv *Server) saveTLSAssets() error {
	if srv.Member.PeerCertPath != "" {
		if srv.Member.PeerCertData == "" {
			return fmt.Errorf("got empty data for %q", srv.Member.PeerCertPath)
		}
		if err := ioutil.WriteFile(srv.Member.PeerCertPath, []byte(srv.Member.PeerCertData), 0644); err != nil {
			return err
		}
	}
	if srv.Member.PeerKeyPath != "" {
		if srv.Member.PeerKeyData == "" {
			return fmt.Errorf("got empty data for %q", srv.Member.PeerKeyPath)
		}
		if err := ioutil.WriteFile(srv.Member.PeerKeyPath, []byte(srv.Member.PeerKeyData), 0644); err != nil {
			return err
		}
	}
	if srv.Member.PeerTrustedCAPath != "" {
		if srv.Member.PeerTrustedCAData == "" {
			return fmt.Errorf("got empty data for %q", srv.Member.PeerTrustedCAPath)
		}
		if err := ioutil.WriteFile(srv.Member.PeerTrustedCAPath, []byte(srv.Member.PeerTrustedCAData), 0644); err != nil {
			return err
		}
	}
	if srv.Member.PeerCertPath != "" &&
		srv.Member.PeerKeyPath != "" &&
		srv.Member.PeerTrustedCAPath != "" {
		srv.lg.Info(
			"wrote",
			zap.String("peer-cert", srv.Member.PeerCertPath),
			zap.String("peer-key", srv.Member.PeerKeyPath),
			zap.String("peer-trusted-ca", srv.Member.PeerTrustedCAPath),
		)
	}

	if srv.Member.ClientCertPath != "" {
		if srv.Member.ClientCertData == "" {
			return fmt.Errorf("got empty data for %q", srv.Member.ClientCertPath)
		}
		if err := ioutil.WriteFile(srv.Member.ClientCertPath, []byte(srv.Member.ClientCertData), 0644); err != nil {
			return err
		}
	}
	if srv.Member.ClientKeyPath != "" {
		if srv.Member.ClientKeyData == "" {
			return fmt.Errorf("got empty data for %q", srv.Member.ClientKeyPath)
		}
		if err := ioutil.WriteFile(srv.Member.ClientKeyPath, []byte(srv.Member.ClientKeyData), 0644); err != nil {
			return err
		}
	}
	if srv.Member.ClientTrustedCAPath != "" {
		if srv.Member.ClientTrustedCAData == "" {
			return fmt.Errorf("got empty data for %q", srv.Member.ClientTrustedCAPath)
		}
		if err := ioutil.WriteFile(srv.Member.ClientTrustedCAPath, []byte(srv.Member.ClientTrustedCAData), 0644); err != nil {
			return err
		}
	}
	if srv.Member.ClientCertPath != "" &&
		srv.Member.ClientKeyPath != "" &&
		srv.Member.ClientTrustedCAPath != "" {
		srv.lg.Info(
			"wrote",
			zap.String("client-cert", srv.Member.ClientCertPath),
			zap.String("client-key", srv.Member.ClientKeyPath),
			zap.String("client-trusted-ca", srv.Member.ClientTrustedCAPath),
		)
	}
	return nil
}

func (srv *Server) loadAutoTLSAssets() error {
	if srv.Member.Etcd.PeerAutoTLS {
		// in case of slow disk
		time.Sleep(time.Second)

		fdir := filepath.Join(srv.Member.Etcd.DataDir, "fixtures", "peer")

		srv.lg.Info(
			"loading peer auto TLS assets",
			zap.String("dir", fdir),
			zap.String("endpoint", srv.EtcdClientEndpoint),
		)

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
			"loaded peer auto TLS assets",
			zap.String("peer-cert-path", certPath),
			zap.Int("peer-cert-length", len(certData)),
			zap.String("peer-key-path", keyPath),
			zap.Int("peer-key-length", len(keyData)),
		)
	}

	if srv.Member.Etcd.ClientAutoTLS {
		// in case of slow disk
		time.Sleep(time.Second)

		fdir := filepath.Join(srv.Member.Etcd.DataDir, "fixtures", "client")

		srv.lg.Info(
			"loading client TLS assets",
			zap.String("dir", fdir),
			zap.String("endpoint", srv.EtcdClientEndpoint),
		)

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
			zap.String("client-cert-path", certPath),
			zap.Int("client-cert-length", len(certData)),
			zap.String("client-key-path", keyPath),
			zap.Int("client-key-length", len(keyData)),
		)
	}

	return nil
}

func (srv *Server) handle_INITIAL_START_ETCD(req *rpcpb.Request) (*rpcpb.Response, error) {
	if srv.last != rpcpb.Operation_NOT_STARTED {
		return &rpcpb.Response{
			Success: false,
			Status:  fmt.Sprintf("%q is not valid; last server operation was %q", rpcpb.Operation_INITIAL_START_ETCD.String(), srv.last.String()),
			Member:  req.Member,
		}, nil
	}

	err := fileutil.TouchDirAll(srv.Member.BaseDir)
	if err != nil {
		return nil, err
	}
	srv.lg.Info("created base directory", zap.String("path", srv.Member.BaseDir))

	if srv.etcdServer == nil {
		if err = srv.createEtcdLogFile(); err != nil {
			return nil, err
		}
	}

	if err = srv.saveTLSAssets(); err != nil {
		return nil, err
	}
	if err = srv.creatEtcd(false, req.Member.Failpoints); err != nil {
		return nil, err
	}
	if err = srv.runEtcd(); err != nil {
		return nil, err
	}
	if err = srv.loadAutoTLSAssets(); err != nil {
		return nil, err
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "start etcd PASS",
		Member:  srv.Member,
	}, nil
}

func (srv *Server) handle_RESTART_ETCD(req *rpcpb.Request) (*rpcpb.Response, error) {
	var err error
	if !fileutil.Exist(srv.Member.BaseDir) {
		err = fileutil.TouchDirAll(srv.Member.BaseDir)
		if err != nil {
			return nil, err
		}
	}

	if err = srv.saveTLSAssets(); err != nil {
		return nil, err
	}
	if err = srv.creatEtcd(false, req.Member.Failpoints); err != nil {
		return nil, err
	}
	if err = srv.runEtcd(); err != nil {
		return nil, err
	}
	if err = srv.loadAutoTLSAssets(); err != nil {
		return nil, err
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "restart etcd PASS",
		Member:  srv.Member,
	}, nil
}

func (srv *Server) handle_SIGTERM_ETCD() (*rpcpb.Response, error) {
	if err := srv.stopEtcd(syscall.SIGTERM); err != nil {
		return nil, err
	}

	if srv.etcdServer != nil {
		srv.etcdServer.GetLogger().Sync()
	} else {
		srv.etcdLogFile.Sync()
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "killed etcd",
	}, nil
}

func (srv *Server) handle_SIGQUIT_ETCD_AND_REMOVE_DATA() (*rpcpb.Response, error) {
	err := srv.stopEtcd(syscall.SIGQUIT)
	if err != nil {
		return nil, err
	}

	if srv.etcdServer != nil {
		srv.etcdServer.GetLogger().Sync()
	} else {
		srv.etcdLogFile.Sync()
		srv.etcdLogFile.Close()
	}

	// for debugging purposes, rename instead of removing
	if err = os.RemoveAll(srv.Member.BaseDir + ".backup"); err != nil {
		return nil, err
	}
	if err = os.Rename(srv.Member.BaseDir, srv.Member.BaseDir+".backup"); err != nil {
		return nil, err
	}
	srv.lg.Info(
		"renamed",
		zap.String("base-dir", srv.Member.BaseDir),
		zap.String("new-dir", srv.Member.BaseDir+".backup"),
	)

	// create a new log file for next new member restart
	if !fileutil.Exist(srv.Member.BaseDir) {
		err = fileutil.TouchDirAll(srv.Member.BaseDir)
		if err != nil {
			return nil, err
		}
	}

	return &rpcpb.Response{
		Success: true,
		Status:  "killed etcd and removed base directory",
	}, nil
}

func (srv *Server) handle_SAVE_SNAPSHOT() (*rpcpb.Response, error) {
	err := srv.Member.SaveSnapshot(srv.lg)
	if err != nil {
		return nil, err
	}
	return &rpcpb.Response{
		Success:      true,
		Status:       "saved snapshot",
		SnapshotInfo: srv.Member.SnapshotInfo,
	}, nil
}

func (srv *Server) handle_RESTORE_RESTART_FROM_SNAPSHOT(req *rpcpb.Request) (resp *rpcpb.Response, err error) {
	err = srv.Member.RestoreSnapshot(srv.lg)
	if err != nil {
		return nil, err
	}
	resp, err = srv.handle_RESTART_FROM_SNAPSHOT(req)
	if resp != nil && err == nil {
		resp.Status = "restored snapshot and " + resp.Status
	}
	return resp, err
}

func (srv *Server) handle_RESTART_FROM_SNAPSHOT(req *rpcpb.Request) (resp *rpcpb.Response, err error) {
	if err = srv.saveTLSAssets(); err != nil {
		return nil, err
	}
	if err = srv.creatEtcd(true, req.Member.Failpoints); err != nil {
		return nil, err
	}
	if err = srv.runEtcd(); err != nil {
		return nil, err
	}
	if err = srv.loadAutoTLSAssets(); err != nil {
		return nil, err
	}

	return &rpcpb.Response{
		Success:      true,
		Status:       "restarted etcd from snapshot",
		SnapshotInfo: srv.Member.SnapshotInfo,
	}, nil
}

func (srv *Server) handle_SIGQUIT_ETCD_AND_ARCHIVE_DATA() (*rpcpb.Response, error) {
	err := srv.stopEtcd(syscall.SIGQUIT)
	if err != nil {
		return nil, err
	}

	if srv.etcdServer != nil {
		srv.etcdServer.GetLogger().Sync()
	} else {
		srv.etcdLogFile.Sync()
		srv.etcdLogFile.Close()
	}

	// TODO: support separate WAL directory
	if err = archive(
		srv.Member.BaseDir,
		srv.Member.Etcd.LogOutputs[0],
		srv.Member.Etcd.DataDir,
	); err != nil {
		return nil, err
	}
	srv.lg.Info("archived data", zap.String("base-dir", srv.Member.BaseDir))

	if srv.etcdServer == nil {
		if err = srv.createEtcdLogFile(); err != nil {
			return nil, err
		}
	}

	srv.lg.Info("cleaning up page cache")
	if err := cleanPageCache(); err != nil {
		srv.lg.Warn("failed to clean up page cache", zap.String("error", err.Error()))
	}
	srv.lg.Info("cleaned up page cache")

	return &rpcpb.Response{
		Success: true,
		Status:  "cleaned up etcd",
	}, nil
}

// stop proxy, etcd, delete data directory
func (srv *Server) handle_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT() (*rpcpb.Response, error) {
	err := srv.stopEtcd(syscall.SIGQUIT)
	if err != nil {
		return nil, err
	}

	if srv.etcdServer != nil {
		srv.etcdServer.GetLogger().Sync()
	} else {
		srv.etcdLogFile.Sync()
		srv.etcdLogFile.Close()
	}

	err = os.RemoveAll(srv.Member.BaseDir)
	if err != nil {
		return nil, err
	}
	srv.lg.Info("removed base directory", zap.String("dir", srv.Member.BaseDir))

	// stop agent server
	srv.Stop()

	return &rpcpb.Response{
		Success: true,
		Status:  "destroyed etcd and agent",
	}, nil
}

func (srv *Server) handle_BLACKHOLE_PEER_PORT_TX_RX() *rpcpb.Response {
	for port, px := range srv.advertisePeerPortToProxy {
		srv.lg.Info("blackholing", zap.Int("peer-port", port))
		px.BlackholeTx()
		px.BlackholeRx()
		srv.lg.Info("blackholed", zap.Int("peer-port", port))
	}
	return &rpcpb.Response{
		Success: true,
		Status:  "blackholed peer port tx/rx",
	}
}

func (srv *Server) handle_UNBLACKHOLE_PEER_PORT_TX_RX() *rpcpb.Response {
	for port, px := range srv.advertisePeerPortToProxy {
		srv.lg.Info("unblackholing", zap.Int("peer-port", port))
		px.UnblackholeTx()
		px.UnblackholeRx()
		srv.lg.Info("unblackholed", zap.Int("peer-port", port))
	}
	return &rpcpb.Response{
		Success: true,
		Status:  "unblackholed peer port tx/rx",
	}
}

func (srv *Server) handle_DELAY_PEER_PORT_TX_RX() *rpcpb.Response {
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
		Status:  "delayed peer port tx/rx",
	}
}

func (srv *Server) handle_UNDELAY_PEER_PORT_TX_RX() *rpcpb.Response {
	for port, px := range srv.advertisePeerPortToProxy {
		srv.lg.Info("undelaying", zap.Int("peer-port", port))
		px.UndelayTx()
		px.UndelayRx()
		srv.lg.Info("undelayed", zap.Int("peer-port", port))
	}
	return &rpcpb.Response{
		Success: true,
		Status:  "undelayed peer port tx/rx",
	}
}
