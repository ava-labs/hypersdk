// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/rs/cors"
	"go.uber.org/zap"
)

var _ Server = (*server)(nil)

type PathAdder interface {
	// AddRoute registers a route to a handler.
	AddRoute(handler http.Handler, base, endpoint string) error
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

func (s *server) AddRoute(handler http.Handler, base, endpoint string) error {
	url := fmt.Sprintf("%s/%s", s.baseURL, base)
	s.log.Info("adding route",
		zap.String("url", url),
		zap.String("endpoint", endpoint),
	)
	return s.router.AddRouter(url, endpoint, handler)
}

func (s *server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	err := s.srv.Shutdown(ctx)
	cancel()

	// If shutdown times out, make sure the server is still shutdown.
	_ = s.srv.Close()
	return err
}
