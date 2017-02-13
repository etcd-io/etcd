// Copyright 2016 The etcd Authors
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

package grpcproxy

import (
	"errors"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var errAlreadySentHeader = errors.New("grpcproxy: already send header")

type ws2wc struct{ wserv pb.WatchServer }

func WatchServerToWatchClient(wserv pb.WatchServer) pb.WatchClient {
	return &ws2wc{wserv}
}

func (s *ws2wc) Watch(ctx context.Context, opts ...grpc.CallOption) (pb.Watch_WatchClient, error) {
	// ch1 is buffered so server can send error on close
	ch1, ch2 := make(chan interface{}, 1), make(chan interface{})
	headerc, trailerc := make(chan metadata.MD, 1), make(chan metadata.MD, 1)

	cctx, ccancel := context.WithCancel(ctx)
	cli := &chanStream{recvc: ch1, sendc: ch2, ctx: cctx, cancel: ccancel}
	wclient := &ws2wcClientStream{chanClientStream{headerc, trailerc, cli}}

	sctx, scancel := context.WithCancel(ctx)
	srv := &chanStream{recvc: ch2, sendc: ch1, ctx: sctx, cancel: scancel}
	wserver := &ws2wcServerStream{chanServerStream{headerc, trailerc, srv, nil}}
	go func() {
		if err := s.wserv.Watch(wserver); err != nil {
			select {
			case srv.sendc <- err:
			case <-sctx.Done():
			case <-cctx.Done():
			}
		}
		scancel()
		ccancel()
	}()
	return wclient, nil
}

// ws2wcClientStream implements Watch_WatchClient
type ws2wcClientStream struct{ chanClientStream }

// ws2wcServerStream implements Watch_WatchServer
type ws2wcServerStream struct{ chanServerStream }

func (s *ws2wcClientStream) Send(wr *pb.WatchRequest) error {
	return s.SendMsg(wr)
}
func (s *ws2wcClientStream) Recv() (*pb.WatchResponse, error) {
	var v interface{}
	if err := s.RecvMsg(&v); err != nil {
		return nil, err
	}
	return v.(*pb.WatchResponse), nil
}

func (s *ws2wcServerStream) Send(wr *pb.WatchResponse) error {
	return s.SendMsg(wr)
}
func (s *ws2wcServerStream) Recv() (*pb.WatchRequest, error) {
	var v interface{}
	if err := s.RecvMsg(&v); err != nil {
		return nil, err
	}
	return v.(*pb.WatchRequest), nil
}
