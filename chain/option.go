// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

type Options struct {
	TransactionManagerFactory TransactionManagerFactory
}

type Option func(*Options)

func WithTransactionManagerFactory(factory TransactionManagerFactory) Option {
	return func(opts *Options) {
		opts.TransactionManagerFactory = factory
	}
}
