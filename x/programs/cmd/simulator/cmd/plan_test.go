// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateCallParams(t *testing.T) {
	// Using a key not yet created must return an error
	// This test creates a simulator to initialize the db we need to use
	ctx := context.Background()
	newSimulator := func() *Simulator {
		logLevel := "error"
		enableWriterDisplaying := true
		cleanup := true
		return &Simulator{
			logLevel:               &logLevel,
			enableWriterDisplaying: &enableWriterDisplaying,
			cleanup:                &cleanup,
		}
	}
	s := newSimulator()
	require.NoError(t, s.Init())
	defer s.manageCleanup(ctx)
	cmd := &runCmd{}
	_, err := cmd.createCallParams(ctx, s.db, []Parameter{{Type: KeyEd25519, Value: []byte("alice")}}, EndpointExecute)
	require.ErrorIs(t, err, ErrNamedKeyNotFound)
	_, err = keyCreateFunc(ctx, s.db, "alice")
	require.NoError(t, err)
	_, err = cmd.createCallParams(ctx, s.db, []Parameter{{Type: KeyEd25519, Value: []byte("alice")}}, EndpointExecute)
	require.NoError(t, err)
}
