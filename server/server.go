// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/NYTimes/gziphandler"

	"github.com/rs/cors"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
)

var (
	errUnknownLockOption = errors.New("invalid lock options")

	_ Server = (*server)(nil)
)

type PathAdder interface {
	// AddRoute registers a route to a handler.
	AddRoute(handler *common.HTTPHandler, lock *sync.RWMutex, base, endpoint string) error
}

// Server maintains the HTTP router
type Server interface {
	PathAdder
	// Dispatch starts the API server
	Dispatch() error
	// Shutdown this server
	Shutdown() error
}

type HTTPConfig struct {
	ReadTimeout       time.Duration `json:"readTimeout"`
	ReadHeaderTimeout time.Duration `json:"readHeaderTimeout"`
	WriteTimeout      time.Duration `json:"writeHeaderTimeout"`
	IdleTimeout       time.Duration `json:"idleTimeout"`
}

type server struct {
	baseURL string

	// log this server writes to
	log logging.Logger

	shutdownTimeout time.Duration

	// Maps endpoints to handlers
	router *router

	srv *http.Server

	// Listener used to serve traffic
	listener net.Listener
}

// New returns an instance of a Server.
func New(
	baseURL string,
	log logging.Logger,
	listener net.Listener,
	httpConfig HTTPConfig,
	allowedOrigins []string,
	allowedHosts []string,
	shutdownTimeout time.Duration,
	wrappers ...Wrapper,
) (Server, error) {
	router := newRouter()
	allowedHostsHandler := filterInvalidHosts(router, allowedHosts)
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowCredentials: true,
	}).Handler(allowedHostsHandler)
	gzipHandler := gziphandler.GzipHandler(corsHandler)
	var handler http.Handler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gzipHandler.ServeHTTP(w, r)
		},
	)

	for _, wrapper := range wrappers {
		handler = wrapper.WrapHandler(handler)
	}

	log.Info("API created",
		zap.Strings("allowedOrigins", allowedOrigins),
	)

	return &server{
		baseURL:         baseURL,
		log:             log,
		shutdownTimeout: shutdownTimeout,
		router:          router,
		srv: &http.Server{
			Handler:           handler,
			ReadTimeout:       httpConfig.ReadTimeout,
			ReadHeaderTimeout: httpConfig.ReadHeaderTimeout,
			WriteTimeout:      httpConfig.WriteTimeout,
			IdleTimeout:       httpConfig.IdleTimeout,
		},
		listener: listener,
	}, nil
}

func (s *server) Dispatch() error {
	return s.srv.Serve(s.listener)
}

func (s *server) AddRoute(handler *common.HTTPHandler, lock *sync.RWMutex, base, endpoint string) error {
	return s.addRoute(handler, lock, base, endpoint)
}

func (s *server) AddRouteWithReadLock(handler *common.HTTPHandler, lock *sync.RWMutex, base, endpoint string) error {
	s.router.lock.RUnlock()
	defer s.router.lock.RLock()
	return s.addRoute(handler, lock, base, endpoint)
}

func (s *server) addRoute(handler *common.HTTPHandler, lock *sync.RWMutex, base, endpoint string) error {
	url := fmt.Sprintf("%s/%s", s.baseURL, base)
	s.log.Info("adding route",
		zap.String("url", url),
		zap.String("endpoint", endpoint),
	)

	// Apply middleware to grab/release chain's lock before/after calling API method
	h, err := lockMiddleware(
		handler.Handler,
		handler.LockOptions,
		lock,
	)
	if err != nil {
		return err
	}
	return s.router.AddRouter(url, endpoint, h)
}

// Wraps a handler by grabbing and releasing a lock before calling the handler.
func lockMiddleware(
	handler http.Handler,
	lockOption common.LockOption,
	lock *sync.RWMutex,
) (http.Handler, error) {
	switch lockOption {
	case common.WriteLock:
		return middlewareHandler{
			before:  lock.Lock,
			after:   lock.Unlock,
			handler: handler,
		}, nil
	case common.ReadLock:
		return middlewareHandler{
			before:  lock.RLock,
			after:   lock.RUnlock,
			handler: handler,
		}, nil
	case common.NoLock:
		return handler, nil
	default:
		return nil, errUnknownLockOption
	}
}

func (s *server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	err := s.srv.Shutdown(ctx)
	cancel()

	// If shutdown times out, make sure the server is still shutdown.
	_ = s.srv.Close()
	return err
}
