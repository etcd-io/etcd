// Copyright 2015 The etcd Authors
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

package embed

import (
	"context"
	"fmt"
	"io/ioutil"
	defaultLog "log"
	"net"
	"net/http"
	"strings"

	"go.etcd.io/etcd/etcdserver"
	"go.etcd.io/etcd/etcdserver/api/v3client"
	"go.etcd.io/etcd/etcdserver/api/v3election"
	"go.etcd.io/etcd/etcdserver/api/v3election/v3electionpb"
	v3electiongw "go.etcd.io/etcd/etcdserver/api/v3election/v3electionpb/gw"
	"go.etcd.io/etcd/etcdserver/api/v3lock"
	"go.etcd.io/etcd/etcdserver/api/v3lock/v3lockpb"
	v3lockgw "go.etcd.io/etcd/etcdserver/api/v3lock/v3lockpb/gw"
	"go.etcd.io/etcd/etcdserver/api/v3rpc"
	etcdservergw "go.etcd.io/etcd/etcdserver/etcdserverpb/gw"
	"go.etcd.io/etcd/pkg/debugutil"
	"go.etcd.io/etcd/pkg/httputil"
	"go.etcd.io/etcd/pkg/transport"

	gw "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/soheilhy/cmux"
	"github.com/tmc/grpc-websocket-proxy/wsproxy"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/trace"
	"google.golang.org/grpc"
)

type serveCtx struct {
	lg *zap.Logger
	l  net.Listener

	scheme   string
	addr     string
	network  string
	secure   bool
	insecure bool
	httpOnly bool

	ctx    context.Context
	cancel context.CancelFunc

	userHandlers    map[string]http.Handler
	serviceRegister func(*grpc.Server)
	serversC        chan *servers
}

type servers struct {
	secure bool
	grpc   *grpc.Server
	http   *http.Server
}

func newServeCtx(lg *zap.Logger) *serveCtx {
	ctx, cancel := context.WithCancel(context.Background())
	return &serveCtx{
		lg:           lg,
		ctx:          ctx,
		cancel:       cancel,
		userHandlers: make(map[string]http.Handler),
		serversC:     make(chan *servers, 2), // in case sctx.insecure,sctx.secure true
	}
}

// serve accepts incoming connections on the listener l,
// creating a new service goroutine for each. The service goroutines
// read requests and then call handler to reply to them.
func (sctx *serveCtx) serve(
	s *etcdserver.EtcdServer,
	tlsinfo *transport.TLSInfo,
	handler http.Handler,
	errHandler func(error),
	grpcDialForRestGatewayBackends func(ctx context.Context) (*grpc.ClientConn, error),
	splitHttp bool,
	gopts ...grpc.ServerOption) (err error) {
	logger := defaultLog.New(ioutil.Discard, "etcdhttp", 0)
	<-s.ReadyNotify()

	if sctx.lg == nil {
		plog.Info("ready to serve client requests")
	}

	m := cmux.New(sctx.l)
	var server func() error
	onlyGRPC := splitHttp && !sctx.httpOnly
	onlyHttp := splitHttp && sctx.httpOnly
	grpcEnabled := !onlyHttp
	httpEnabled := !onlyGRPC

	v3c := v3client.New(s)
	servElection := v3election.NewElectionServer(v3c)
	servLock := v3lock.NewLockServer(v3c)

	// Make sure serversC is closed even if we prematurely exit the function.
	defer close(sctx.serversC)
	var gwmux *gw.ServeMux
	if s.Cfg.EnableGRPCGateway {
		// GRPC gateway connects to grpc server via connection provided by grpc dial.
		gwmux, err = sctx.registerGateway(grpcDialForRestGatewayBackends)
		if err != nil {
			sctx.lg.Error("registerGateway failed", zap.Error(err))
			return err
		}
	}
	var traffic string
	switch {
	case onlyGRPC:
		traffic = "grpc"
	case onlyHttp:
		traffic = "http"
	default:
		traffic = "grpc+http"
	}

	if sctx.insecure {
		var gs *grpc.Server
		var srv *http.Server
		if httpEnabled {
			httpmux := sctx.createMux(gwmux, handler)
			srv = &http.Server{
				Handler:  createAccessController(sctx.lg, s, httpmux),
				ErrorLog: logger, // do not log user error
			}
			if err := configureHttpServer(srv, s.Cfg); err != nil {
				sctx.lg.Error("Configure http server failed", zap.Error(err))
				return err
			}
		}
		if grpcEnabled {
			gs = v3rpc.Server(s, nil, nil, gopts...)
			v3electionpb.RegisterElectionServer(gs, servElection)
			v3lockpb.RegisterLockServer(gs, servLock)
			if sctx.serviceRegister != nil {
				sctx.serviceRegister(gs)
			}
			defer func(gs *grpc.Server) {
				if err == nil {
					return
				}

				if sctx.lg != nil {
					sctx.lg.Warn("stopping insecure grpc server due to error", zap.Error(err))
				} else {
					plog.Warningf("stopping insecure grpc server due to error: %s", err)
				}

				gs.Stop()

				if sctx.lg != nil {
					sctx.lg.Warn("stopped insecure grpc server due to error", zap.Error(err))
				} else {
					plog.Warningf("stopped insecure grpc server due to error: %s", err)
				}
			}(gs)
		}
		if onlyGRPC {
			server = func() error {
				return gs.Serve(sctx.l)
			}
		} else {
			server = m.Serve

			httpl := m.Match(cmux.HTTP1())
			go func(srvhttp *http.Server, tlsLis net.Listener) {
				errHandler(srvhttp.Serve(tlsLis))
			}(srv, httpl)

			if grpcEnabled {
				grpcl := m.Match(cmux.HTTP2())
				go func(gs *grpc.Server, l net.Listener) {
					errHandler(gs.Serve(l))
				}(gs, grpcl)
			}
		}

		sctx.serversC <- &servers{grpc: gs, http: srv}
		if sctx.lg != nil {
			sctx.lg.Info(
				"serving client traffic insecurely; this is strongly discouraged!",
				zap.String("traffic", traffic),
				zap.String("address", sctx.l.Addr().String()),
			)
		} else {
			plog.Noticef("serving insecure client requests on %s, this is strongly discouraged!", sctx.l.Addr().String())
		}
	}

	if sctx.secure {
		var gs *grpc.Server
		var srv *http.Server

		tlscfg, tlsErr := tlsinfo.ServerConfig()
		if tlsErr != nil {
			return tlsErr
		}

		if grpcEnabled {
			gs = v3rpc.Server(s, tlscfg, nil, gopts...)
			v3electionpb.RegisterElectionServer(gs, servElection)
			v3lockpb.RegisterLockServer(gs, servLock)
			if sctx.serviceRegister != nil {
				sctx.serviceRegister(gs)
			}
			defer func(gs *grpc.Server) {
				if err == nil {
					return
				}

				if sctx.lg != nil {
					sctx.lg.Warn("stopping secure grpc server due to error", zap.Error(err))
				} else {
					plog.Warningf("stopping secure grpc server due to error: %s", err)
				}

				gs.Stop()

				if sctx.lg != nil {
					sctx.lg.Warn("stopped secure grpc server due to error", zap.Error(err))
				} else {
					plog.Warningf("stopped secure grpc server due to error: %s", err)
				}
			}(gs)
		}
		if httpEnabled {
			if grpcEnabled {
				handler = grpcHandlerFunc(gs, handler)
			}
			httpmux := sctx.createMux(gwmux, handler)

			srv = &http.Server{
				Handler:   createAccessController(sctx.lg, s, httpmux),
				TLSConfig: tlscfg,
				ErrorLog:  logger, // do not log user error
			}
			if err := configureHttpServer(srv, s.Cfg); err != nil {
				sctx.lg.Error("Configure https server failed", zap.Error(err))
				return err
			}
		}

		if onlyGRPC {
			server = func() error { return gs.Serve(sctx.l) }
		} else {
			server = m.Serve

			tlsl, err := transport.NewTLSListener(m.Match(cmux.Any()), tlsinfo)
			if err != nil {
				return err
			}
			go func(srvhttp *http.Server, tlsl net.Listener) {
				errHandler(srvhttp.Serve(tlsl))
			}(srv, tlsl)
		}

		sctx.serversC <- &servers{secure: true, grpc: gs, http: srv}
		if sctx.lg != nil {
			sctx.lg.Info(
				"serving client traffic securely",
				zap.String("traffic", traffic),
				zap.String("address", sctx.l.Addr().String()),
			)
		} else {
			plog.Infof("serving client requests on %s", sctx.l.Addr().String())
		}
	}

	return server()
}

func configureHttpServer(srv *http.Server, cfg etcdserver.ServerConfig) error {
	// todo (ahrtr): should we support configuring other parameters in the future as well?
	return http2.ConfigureServer(srv, &http2.Server{
		MaxConcurrentStreams: cfg.MaxConcurrentStreams,
	})
}

// grpcHandlerFunc returns an http.Handler that delegates to grpcServer on incoming gRPC
// connections or otherHandler otherwise. Given in gRPC docs.
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	if otherHandler == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			grpcServer.ServeHTTP(w, r)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}

type registerHandlerFunc func(context.Context, *gw.ServeMux, *grpc.ClientConn) error

func (sctx *serveCtx) registerGateway(dial func(ctx context.Context) (*grpc.ClientConn, error)) (*gw.ServeMux, error) {
	ctx := sctx.ctx

	conn, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	gwmux := gw.NewServeMux()

	handlers := []registerHandlerFunc{
		etcdservergw.RegisterKVHandler,
		etcdservergw.RegisterWatchHandler,
		etcdservergw.RegisterLeaseHandler,
		etcdservergw.RegisterClusterHandler,
		etcdservergw.RegisterMaintenanceHandler,
		etcdservergw.RegisterAuthHandler,
		v3lockgw.RegisterLockHandler,
		v3electiongw.RegisterElectionHandler,
	}
	for _, h := range handlers {
		if err := h(ctx, gwmux, conn); err != nil {
			return nil, err
		}
	}
	go func() {
		<-ctx.Done()
		if cerr := conn.Close(); cerr != nil {
			if sctx.lg != nil {
				sctx.lg.Warn(
					"failed to close connection",
					zap.String("address", sctx.l.Addr().String()),
					zap.Error(cerr),
				)
			} else {
				plog.Warningf("failed to close conn to %s: %v", sctx.l.Addr().String(), cerr)
			}
		}
	}()

	return gwmux, nil
}

type wsProxyZapLogger struct {
	*zap.Logger
}

func (w wsProxyZapLogger) Warnln(i ...interface{}) {
	w.Warn(fmt.Sprint(i...))
}

func (w wsProxyZapLogger) Debugln(i ...interface{}) {
	w.Debug(fmt.Sprint(i...))
}

func (sctx *serveCtx) createMux(gwmux *gw.ServeMux, handler http.Handler) *http.ServeMux {
	httpmux := http.NewServeMux()
	for path, h := range sctx.userHandlers {
		httpmux.Handle(path, h)
	}

	if gwmux != nil {
		httpmux.Handle(
			"/v3/",
			wsproxy.WebsocketProxy(
				gwmux,
				wsproxy.WithRequestMutator(
					// Default to the POST method for streams
					func(_ *http.Request, outgoing *http.Request) *http.Request {
						outgoing.Method = "POST"
						return outgoing
					},
				),
				wsproxy.WithMaxRespBodyBufferSize(0x7fffffff),
				wsproxy.WithLogger(wsProxyZapLogger{sctx.lg}),
			),
		)
	}
	if handler != nil {
		httpmux.Handle("/", handler)
	}
	return httpmux
}

// createAccessController wraps HTTP multiplexer:
// - mutate gRPC gateway request paths
// - check hostname whitelist
// client HTTP requests goes here first
func createAccessController(lg *zap.Logger, s *etcdserver.EtcdServer, mux *http.ServeMux) http.Handler {
	return &accessController{lg: lg, s: s, mux: mux}
}

type accessController struct {
	lg  *zap.Logger
	s   *etcdserver.EtcdServer
	mux *http.ServeMux
}

func (ac *accessController) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// redirect for backward compatibilities
	if req != nil && req.URL != nil && strings.HasPrefix(req.URL.Path, "/v3beta/") {
		req.URL.Path = strings.Replace(req.URL.Path, "/v3beta/", "/v3/", 1)
	}

	if req.TLS == nil { // check origin if client connection is not secure
		host := httputil.GetHostname(req)
		if !ac.s.AccessController.IsHostWhitelisted(host) {
			if ac.lg != nil {
				ac.lg.Warn(
					"rejecting HTTP request to prevent DNS rebinding attacks",
					zap.String("host", host),
				)
			} else {
				plog.Warningf("rejecting HTTP request from %q to prevent DNS rebinding attacks", host)
			}
			// TODO: use Go's "http.StatusMisdirectedRequest" (421)
			// https://github.com/golang/go/commit/4b8a7eafef039af1834ef9bfa879257c4a72b7b5
			http.Error(rw, errCVE20185702(host), 421)
			return
		}
	} else if ac.s.Cfg.ClientCertAuthEnabled && ac.s.Cfg.EnableGRPCGateway &&
		ac.s.AuthStore().IsAuthEnabled() && strings.HasPrefix(req.URL.Path, "/v3/") {
		for _, chains := range req.TLS.VerifiedChains {
			if len(chains) < 1 {
				continue
			}
			if len(chains[0].Subject.CommonName) != 0 {
				http.Error(rw, "CommonName of client sending a request against gateway will be ignored and not used as expected", 400)
				return
			}
		}
	}

	// Write CORS header.
	if ac.s.AccessController.OriginAllowed("*") {
		addCORSHeader(rw, "*")
	} else if origin := req.Header.Get("Origin"); ac.s.OriginAllowed(origin) {
		addCORSHeader(rw, origin)
	}

	if req.Method == "OPTIONS" {
		rw.WriteHeader(http.StatusOK)
		return
	}

	ac.mux.ServeHTTP(rw, req)
}

// addCORSHeader adds the correct cors headers given an origin
func addCORSHeader(w http.ResponseWriter, origin string) {
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Headers", "accept, content-type, authorization")
}

// https://github.com/transmission/transmission/pull/468
func errCVE20185702(host string) string {
	return fmt.Sprintf(`
etcd received your request, but the Host header was unrecognized.

To fix this, choose one of the following options:
- Enable TLS, then any HTTPS request will be allowed.
- Add the hostname you want to use to the whitelist in settings.
  - e.g. etcd --host-whitelist %q

This requirement has been added to help prevent "DNS Rebinding" attacks (CVE-2018-5702).
`, host)
}

// WrapCORS wraps existing handler with CORS.
// TODO: deprecate this after v2 proxy deprecate
func WrapCORS(cors map[string]struct{}, h http.Handler) http.Handler {
	return &corsHandler{
		ac: &etcdserver.AccessController{CORS: cors},
		h:  h,
	}
}

type corsHandler struct {
	ac *etcdserver.AccessController
	h  http.Handler
}

func (ch *corsHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if ch.ac.OriginAllowed("*") {
		addCORSHeader(rw, "*")
	} else if origin := req.Header.Get("Origin"); ch.ac.OriginAllowed(origin) {
		addCORSHeader(rw, origin)
	}

	if req.Method == "OPTIONS" {
		rw.WriteHeader(http.StatusOK)
		return
	}

	ch.h.ServeHTTP(rw, req)
}

func (sctx *serveCtx) registerUserHandler(s string, h http.Handler) {
	if sctx.userHandlers[s] != nil {
		if sctx.lg != nil {
			sctx.lg.Warn("path is already registered by user handler", zap.String("path", s))
		} else {
			plog.Warningf("path %s already registered by user handler", s)
		}
		return
	}
	sctx.userHandlers[s] = h
}

func (sctx *serveCtx) registerPprof() {
	for p, h := range debugutil.PProfHandlers() {
		sctx.registerUserHandler(p, h)
	}
}

func (sctx *serveCtx) registerTrace() {
	reqf := func(w http.ResponseWriter, r *http.Request) { trace.Render(w, r, true) }
	sctx.registerUserHandler("/debug/requests", http.HandlerFunc(reqf))
	evf := func(w http.ResponseWriter, r *http.Request) { trace.RenderEvents(w, r, true) }
	sctx.registerUserHandler("/debug/events", http.HandlerFunc(evf))
}
