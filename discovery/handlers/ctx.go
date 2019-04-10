package handlers

import (
	"context"
	"net/http"
)

type ContextHandler interface {
	ServeHTTPContext(context.Context, http.ResponseWriter, *http.Request)
}

type ContextHandlerFunc func(context.Context, http.ResponseWriter, *http.Request)

func (f ContextHandlerFunc) ServeHTTPContext(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	f(ctx, w, req)
}

type ContextAdapter struct {
	Ctx     context.Context
	Handler ContextHandler
}

func (ca *ContextAdapter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ca.Handler.ServeHTTPContext(ca.Ctx, w, req)
}

type key int

const (
	stateKey key = iota
)

func With(h ContextHandler, st *State) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, req *http.Request) {
		ctx = context.WithValue(ctx, stateKey, st)
		h.ServeHTTPContext(ctx, w, req)
	})
}
