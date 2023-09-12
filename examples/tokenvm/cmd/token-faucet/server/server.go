package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/NYTimes/gziphandler"

	"github.com/rs/cors"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
)

const baseURL = "/ext"

var (
	errUnknownLockOption = errors.New("invalid lock options")

	_ PathAdder = readPathAdder{}
	_ Server    = (*server)(nil)
)

type PathAdder interface {
	// AddRoute registers a route to a handler.
	AddRoute(handler *common.HTTPHandler, lock *sync.RWMutex, base, endpoint string) error

	// AddAliases registers aliases to the server
	AddAliases(endpoint string, aliases ...string) error
}

type PathAdderWithReadLock interface {
	// AddRouteWithReadLock registers a route to a handler assuming the http
	// read lock is currently held.
	AddRouteWithReadLock(handler *common.HTTPHandler, lock *sync.RWMutex, base, endpoint string) error

	// AddAliasesWithReadLock registers aliases to the server assuming the http read
	// lock is currently held.
	AddAliasesWithReadLock(endpoint string, aliases ...string) error
}

// Server maintains the HTTP router
type Server interface {
	PathAdder
	PathAdderWithReadLock
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
	log logging.Logger,
	listener net.Listener,
	allowedOrigins []string,
	shutdownTimeout time.Duration,
	httpConfig HTTPConfig,
	allowedHosts []string,
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

func (s *server) RegisterChain(chainName string, ctx *snow.ConsensusContext, vm common.VM) {
	var (
		handlers map[string]*common.HTTPHandler
		err      error
	)

	ctx.Lock.Lock()
	handlers, err = vm.CreateHandlers(context.TODO())
	ctx.Lock.Unlock()
	if err != nil {
		s.log.Error("failed to create handlers",
			zap.String("chainName", chainName),
			zap.Error(err),
		)
		return
	}

	s.log.Verbo("about to add API endpoints",
		zap.Stringer("chainID", ctx.ChainID),
	)
	// all subroutes to a chain begin with "bc/<the chain's ID>"
	defaultEndpoint := path.Join(constants.ChainAliasPrefix, ctx.ChainID.String())

	// Register each endpoint
	for extension, handler := range handlers {
		// Validate that the route being added is valid
		// e.g. "/foo" and "" are ok but "\n" is not
		_, err := url.ParseRequestURI(extension)
		if extension != "" && err != nil {
			s.log.Error("could not add route to chain's API handler",
				zap.String("reason", "route is malformed"),
				zap.Error(err),
			)
			continue
		}
		if err := s.addChainRoute(chainName, handler, ctx, defaultEndpoint, extension); err != nil {
			s.log.Error("error adding route",
				zap.Error(err),
			)
		}
	}
}

func (s *server) addChainRoute(chainName string, handler *common.HTTPHandler, ctx *snow.ConsensusContext, base, endpoint string) error {
	url := fmt.Sprintf("%s/%s", baseURL, base)
	s.log.Info("adding route",
		zap.String("url", url),
		zap.String("endpoint", endpoint),
	)
	// Apply middleware to grab/release chain's lock before/after calling API method
	h, err := lockMiddleware(
		handler.Handler,
		handler.LockOptions,
		&ctx.Lock,
	)
	if err != nil {
		return err
	}
	return s.router.AddRouter(url, endpoint, h)
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
	url := fmt.Sprintf("%s/%s", baseURL, base)
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

func (s *server) AddAliases(endpoint string, aliases ...string) error {
	url := fmt.Sprintf("%s/%s", baseURL, endpoint)
	endpoints := make([]string, len(aliases))
	for i, alias := range aliases {
		endpoints[i] = fmt.Sprintf("%s/%s", baseURL, alias)
	}
	return s.router.AddAlias(url, endpoints...)
}

func (s *server) AddAliasesWithReadLock(endpoint string, aliases ...string) error {
	// This is safe, as the read lock doesn't actually need to be held once the
	// http handler is called. However, it is unlocked later, so this function
	// must end with the lock held.
	s.router.lock.RUnlock()
	defer s.router.lock.RLock()

	return s.AddAliases(endpoint, aliases...)
}

func (s *server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	err := s.srv.Shutdown(ctx)
	cancel()

	// If shutdown times out, make sure the server is still shutdown.
	_ = s.srv.Close()
	return err
}

type readPathAdder struct {
	pather PathAdderWithReadLock
}

func PathWriterFromWithReadLock(pather PathAdderWithReadLock) PathAdder {
	return readPathAdder{
		pather: pather,
	}
}

func (a readPathAdder) AddRoute(handler *common.HTTPHandler, lock *sync.RWMutex, base, endpoint string) error {
	return a.pather.AddRouteWithReadLock(handler, lock, base, endpoint)
}

func (a readPathAdder) AddAliases(endpoint string, aliases ...string) error {
	return a.pather.AddAliasesWithReadLock(endpoint, aliases...)
}
