package main

import (
	"bytes"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/jsonpb"
	"github.com/tmc/grpc-websocket-proxy/examples/cmd/wsechoserver/echoserver"
)

type Server struct{}

func (s *Server) Stream(_ *echoserver.Empty, stream echoserver.EchoService_StreamServer) error {
	start := time.Now()
	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		if err := stream.Send(&echoserver.EchoResponse{
			Message: "hello there!" + fmt.Sprint(time.Now().Sub(start)),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Echo(srv echoserver.EchoService_EchoServer) error {
	for {
		req, err := srv.Recv()
		if err != nil {
			return err
		}
		if err := srv.Send(&echoserver.EchoResponse{
			Message: req.Message + "!",
		}); err != nil {
			return err
		}
	}
}

func (s *Server) Heartbeats(srv echoserver.EchoService_HeartbeatsServer) error {
	go func() {
		for {
			_, err := srv.Recv()
			if err != nil {
				log.Println("Recv() err:", err)
				return
			}
			log.Println("got hb from client")
		}
	}()
	t := time.NewTicker(time.Second * 1)
	for {
		log.Println("sending hb")
		hb := &echoserver.Heartbeat{
			Status: echoserver.Heartbeat_OK,
		}
		b := new(bytes.Buffer)
		if err := (&jsonpb.Marshaler{}).Marshal(b, hb); err != nil {
			log.Println("marshal err:", err)
		}
		log.Println(string(b.Bytes()))
		if err := srv.Send(hb); err != nil {
			return err
		}
		<-t.C
	}
	return nil
}
