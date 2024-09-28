// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package requester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	rpc "github.com/gorilla/rpc/v2/json2"
)

type Option func(*Options)

type Options struct {
	headers     http.Header
	queryParams url.Values
	writer      io.Writer
}

func NewOptions(ops []Option) *Options {
	o := &Options{
		headers:     http.Header{},
		queryParams: url.Values{},
	}
	o.applyOptions(ops)
	return o
}

func (o *Options) applyOptions(ops []Option) {
	for _, op := range ops {
		op(o)
	}
}

func (o *Options) Headers() http.Header {
	return o.headers
}

func (o *Options) QueryParams() url.Values {
	return o.queryParams
}

func WithHeader(key, val string) Option {
	return func(o *Options) {
		o.headers.Set(key, val)
	}
}

func WithQueryParam(key, val string) Option {
	return func(o *Options) {
		o.queryParams.Set(key, val)
	}
}

func WithWriter(w io.Writer) Option {
	return func(o *Options) {
		o.writer = w
	}
}

type EndpointRequester struct {
	cli       *http.Client
	uri, base string
	opts      []Option
}

func New(uri, base string, opts ...Option) *EndpointRequester {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100_000
	t.MaxConnsPerHost = 100_000
	t.MaxIdleConnsPerHost = 100_000

	return &EndpointRequester{
		cli: &http.Client{
			Timeout:   10 * time.Second,
			Transport: t,
		},
		uri:  uri,
		base: base,
		opts: opts,
	}
}

func (e *EndpointRequester) SendRequest(
	ctx context.Context,
	method string,
	params interface{},
	reply interface{},
	options ...Option,
) error {
	uri, err := url.Parse(e.uri)
	if err != nil {
		return err
	}
	options = append(e.opts, options...)
	return SendJSONRequest(
		ctx,
		e.cli,
		uri,
		fmt.Sprintf("%s.%s", e.base, method),
		params,
		reply,
		options...,
	)
}

func SendJSONRequest(
	ctx context.Context,
	cli *http.Client,
	uri *url.URL,
	method string,
	params interface{},
	reply interface{},
	options ...Option,
) error {
	requestBodyBytes, err := rpc.EncodeClientRequest(method, params)
	if err != nil {
		return fmt.Errorf("failed to encode client params: %w", err)
	}

	ops := NewOptions(options)
	uri.RawQuery = ops.queryParams.Encode()

	if ops.writer != nil {
		requestStr := fmt.Sprintf("Request: %s\n%s\n", uri.String(), requestBodyBytes)
		if _, err := ops.writer.Write([]byte(requestStr)); err != nil {
			return err
		}
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		uri.String(),
		bytes.NewBuffer(requestBodyBytes),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	request.Header = ops.headers
	request.Header.Set("Content-Type", "application/json")

	resp, err := cli.Do(request)
	if err != nil {
		return fmt.Errorf("failed to issue request: %w", err)
	}

	// Return an error for any non successful status code
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Drop any error during close to report the original error
		all, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return fmt.Errorf("received status code: %d %s %s", resp.StatusCode, all, uri.String())
	}

	if err := rpc.DecodeClientResponse(resp.Body, reply); err != nil {
		// Drop any error during close to report the original error
		all, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return fmt.Errorf("failed to decode client response: %w %s %s", err, all, uri.String())
	}
	if ops.writer != nil {
		replyBytes, err := json.Marshal(reply)
		if err != nil {
			return err
		}

		if _, err := ops.writer.Write([]byte(fmt.Sprintf("Response: %s\n", replyBytes))); err != nil {
			return err
		}
	}
	return resp.Body.Close()
}
