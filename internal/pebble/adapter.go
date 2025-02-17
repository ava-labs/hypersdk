// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pebble

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
)

var errDBExtension = errors.New("database does not support extensions")

// ExtendedDatabase extends the base Database interface
// While not all database implementations may support these operations natively,
// this interface ensures consistency across different implementations
type ExtendedDatabase interface {
	database.Database
	DeleteRange(start, end []byte) error
}

// fallbackRangeDB provides a generic implementation of ExtendedDatabase that works
// with any Database implementation. It implements range operations using basic
// database operations, ensuring functionality even when the underlying database
// doesn't support range operations natively.
type fallbackRangeDB struct {
	database.Database // Embed the original database to inherit all its methods
}

// DeleteRange implements the ExtendedDatabase interface for databases that don't
// support native range deletion. It performs deletion by iterating through
// the key range and deleting keys in batches to maintain reasonable memory usage.
func (f *fallbackRangeDB) DeleteRange(start, end []byte) error {
	iterator := f.NewIteratorWithStart(start)
	defer iterator.Release()

	b := f.NewBatch()
	for iterator.Next() {
		key := iterator.Key()
		// Check if we've reached the end of our range
		if bytes.Compare(key, end) >= 0 {
			break
		}

		// Add key to the current batch for deletion
		if err := b.Delete(key); err != nil {
			return fmt.Errorf("failed to queue key deletion in batch: %w", err)
		}
	}

	if err := b.Write(); err != nil {
		return fmt.Errorf("failed to write final deletion batch: %w", err)
	}

	// Check if the iterator encountered any errors
	if err := iterator.Error(); err != nil {
		return fmt.Errorf("iterator error during range deletion: %w", err)
	}

	return nil
}

// WithExtendedDatabase wraps a standard Database with extended capabilities
// If the provided database already implements ExtendedDatabase, it is returned as-is.
// Otherwise, it wraps the database with a fallback implementation that provides
// range deletion through iteration and batching.
//
// This function allows code to uniformly handle databases with and without native
// range deletion support, while still benefiting from native implementations
// where available.
//
// Example usage:
//
//	db := memdb.New()  // Some standard Database implementation
//	extendedDB := WithExtendedDatabase(db)
//	// We can use DeleteRange regardless of native support
//	err := extendedDB.DeleteRange(startKey, endKey)
func WithExtendedDatabase(db database.Database) ExtendedDatabase {
	// First check if the database already implements ExtendedDatabase
	if edb, ok := db.(ExtendedDatabase); ok {
		return edb
	}
	// Otherwise, wrap it with our fallback implementation
	return &fallbackRangeDB{Database: db}
}

// AsExtendedDatabase attempts to convert a Database to a ExtendedDatabase, returning
// an error if the database doesn't support range operations and a fallback
// implementation is not desired.
//
// This function is useful when you specifically want to know whether the
// database supports range operations natively, rather than always falling back
// to the iterative implementation.
//
// Example usage:
//
//	edb, err := AsExtendedDatabase(db)
//	if err != nil {
//	    // Handle databases without extensions
//	    return err
//	}
func AsExtendedDatabase(db database.Database) (ExtendedDatabase, error) {
	if rdb, ok := db.(ExtendedDatabase); ok {
		return rdb, nil
	}
	return nil, errDBExtension
}
