// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ethrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	rpc "github.com/gorilla/rpc/v2/json2"
)

type EndpointRequester struct {
	cli       *http.Client
	uri, base string
}

func NewRequester(uri, base string) *EndpointRequester {
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
	}
}

func (e *EndpointRequester) SendRequest(
	ctx context.Context,
	method string,
	params interface{},
	reply interface{},
) error {
	uri, err := url.Parse(e.uri)
	if err != nil {
		return err
	}
	return SendJSONRequest(
		ctx,
		e.cli,
		uri,
		fmt.Sprintf("%s_%s", e.base, method),
		params,
		reply,
	)
}

func SendJSONRequest(
	ctx context.Context,
	cli *http.Client,
	uri *url.URL,
	method string,
	params interface{},
	reply interface{},
) error {
	requestBodyBytes, err := rpc.EncodeClientRequest(method, params)
	if err != nil {
		return fmt.Errorf("failed to encode client params: %w", err)
	}

	uri.RawQuery = url.Values{}.Encode()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		uri.String(),
		bytes.NewBuffer(requestBodyBytes),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	request.Header = http.Header{}
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
	return resp.Body.Close()
}
