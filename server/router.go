// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package server

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

var (
	errUnknownBaseURL  = errors.New("unknown base url")
	errUnknownEndpoint = errors.New("unknown endpoint")
)

type router struct {
	lock   sync.RWMutex
	router *mux.Router

	routeLock sync.Mutex
	routes    map[string]map[string]http.Handler // Maps routes to a handler
}

func newRouter() *router {
	return &router{
		router: mux.NewRouter(),
		routes: make(map[string]map[string]http.Handler),
	}
}

func (r *router) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	r.router.ServeHTTP(writer, request)
}

func (r *router) GetHandler(base, endpoint string) (http.Handler, error) {
	r.routeLock.Lock()
	defer r.routeLock.Unlock()

	urlBase, exists := r.routes[base]
	if !exists {
		return nil, errUnknownBaseURL
	}
	handler, exists := urlBase[endpoint]
	if !exists {
		return nil, errUnknownEndpoint
	}
	return handler, nil
}

func (r *router) AddRouter(base, endpoint string, handler http.Handler) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.routeLock.Lock()
	defer r.routeLock.Unlock()

	return r.addRouter(base, endpoint, handler)
}

func (r *router) addRouter(base, endpoint string, handler http.Handler) error {
	endpoints := r.routes[base]
	if endpoints == nil {
		endpoints = make(map[string]http.Handler)
	}
	url := base + endpoint
	if _, exists := endpoints[endpoint]; exists {
		return fmt.Errorf("failed to create endpoint as %s already exists", url)
	}

	endpoints[endpoint] = handler
	r.routes[base] = endpoints

	// Name routes based on their URL for easy retrieval in the future
	route := r.router.Handle(url, handler)
	if route == nil {
		return fmt.Errorf("failed to create new route for %s", url)
	}
	route.Name(url)
	return nil
}
