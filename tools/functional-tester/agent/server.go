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
	"math"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/tools/functional-tester/rpcpb"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Server implements "rpcpb.TransportServer"
// and other etcd operations as an agent
// no need to lock fields since request operations are
// serialized in tester-side
type Server struct {
	grpcServer *grpc.Server
	logger     *zap.Logger

	network string
	address string
	ln      net.Listener

	rpcpb.TransportServer
	last rpcpb.Operation

	*rpcpb.Member
	*rpcpb.Tester

	etcdCmd     *exec.Cmd
	etcdLogFile *os.File

	// forward incoming advertise URLs traffic to listen URLs
	advertiseClientPortToProxy map[int]transport.Proxy
	advertisePeerPortToProxy   map[int]transport.Proxy
}

// NewServer returns a new agent server.
func NewServer(
	logger *zap.Logger,
	network string,
	address string,
) *Server {
	return &Server{
		logger:  logger,
		network: network,
		address: address,
		last:    rpcpb.Operation_NotStarted,
		advertiseClientPortToProxy: make(map[int]transport.Proxy),
		advertisePeerPortToProxy:   make(map[int]transport.Proxy),
	}
}

const (
	maxRequestBytes   = 1.5 * 1024 * 1024
	grpcOverheadBytes = 512 * 1024
	maxStreams        = math.MaxUint32
	maxSendBytes      = math.MaxInt32
)

// StartServe starts serving agent server.
func (srv *Server) StartServe() error {
	var err error
	srv.ln, err = net.Listen(srv.network, srv.address)
	if err != nil {
		return err
	}

	var opts []grpc.ServerOption
	opts = append(opts, grpc.MaxRecvMsgSize(int(maxRequestBytes+grpcOverheadBytes)))
	opts = append(opts, grpc.MaxSendMsgSize(maxSendBytes))
	opts = append(opts, grpc.MaxConcurrentStreams(maxStreams))
	srv.grpcServer = grpc.NewServer(opts...)

	rpcpb.RegisterTransportServer(srv.grpcServer, srv)

	srv.logger.Info(
		"gRPC server started",
		zap.String("address", srv.address),
		zap.String("listener-address", srv.ln.Addr().String()),
	)
	err = srv.grpcServer.Serve(srv.ln)
	if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
		srv.logger.Info(
			"gRPC server is shut down",
			zap.String("address", srv.address),
			zap.Error(err),
		)
	} else {
		srv.logger.Warn(
			"gRPC server returned with error",
			zap.String("address", srv.address),
			zap.Error(err),
		)
	}

	return err
}

// Stop stops serving gRPC server.
func (srv *Server) Stop() {
	srv.logger.Info("gRPC server stopping", zap.String("address", srv.address))
	srv.grpcServer.Stop()
	srv.logger.Info("gRPC server stopped", zap.String("address", srv.address))
}

// Transport communicates with etcd tester.
func (srv *Server) Transport(stream rpcpb.Transport_TransportServer) (err error) {
	errc := make(chan error)
	go func() {
		for {
			req, err := stream.Recv()
			if err != nil {
				errc <- err
				// TODO: handle error and retry
				return
			}
			if req.Member != nil {
				srv.Member = req.Member
			}
			if req.Tester != nil {
				srv.Tester = req.Tester
			}

			resp, err := srv.handleTesterRequest(req)
			if err != nil {
				errc <- err
				// TODO: handle error and retry
				return
			}

			if err = stream.Send(resp); err != nil {
				errc <- err
				// TODO: handle error and retry
				return
			}
		}
	}()

	select {
	case err = <-errc:
	case <-stream.Context().Done():
		err = stream.Context().Err()
	}
	return err
}
