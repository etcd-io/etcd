package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"

	_ "golang.org/x/net/trace"

	"github.com/golang/glog"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/tmc/grpc-websocket-proxy/examples/cmd/wsechoserver/echoserver"
	"github.com/tmc/grpc-websocket-proxy/wsproxy"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	grpcAddr  = flag.String("grpcaddr", ":8001", "listen grpc addr")
	httpAddr  = flag.String("addr", ":8000", "listen http addr")
	debugAddr = flag.String("debugaddr", ":8002", "listen debug addr")
)

func run() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := listenGRPC(*grpcAddr); err != nil {
		return err
	}

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err := echoserver.RegisterEchoServiceHandlerFromEndpoint(ctx, mux, *grpcAddr, opts)
	if err != nil {
		return err
	}
	go http.ListenAndServe(*debugAddr, nil)
	fmt.Println("listening")
	http.ListenAndServe(*httpAddr, wsproxy.WebsocketProxy(mux))
	return nil
}

func listenGRPC(listenAddr string) error {
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	echoserver.RegisterEchoServiceServer(grpcServer, &Server{})
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Println("serveGRPC err:", err)
		}
	}()
	return nil

}

func main() {
	flag.Parse()
	defer glog.Flush()

	if err := run(); err != nil {
		glog.Fatal(err)
	}
}
