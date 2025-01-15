// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import "github.com/ava-labs/hypersdk/state/shim"

type Options struct {
	executionShim shim.Execution
	// exportStateDiffFunc allows users to override the state diff at the end of block execution
	exportStateDiffFunc ExportStateDiffFunc
	refundFunc          RefundFunc
	dimsModifierFunc    FeeManagerModifierFunc
	resultModifierFunc  ResultModifierFunc
}

type Option func(*Options)

func NewDefaultOptions() *Options {
	return &Options{
		executionShim:       &shim.ExecutionNoOp{},
		exportStateDiffFunc: DefaultExportStateDiff,
		refundFunc:          DefaultRefundFunc,
		dimsModifierFunc:    FeeManagerModifier,
		resultModifierFunc:  DefaultResultModifier,
	}
}

func WithExecutionShim(shim shim.Execution) Option {
	return func(opts *Options) {
		opts.executionShim = shim
	}
}

func WithExportStateDiffFunc(exportStateDiff ExportStateDiffFunc) Option {
	return func(opts *Options) {
		opts.exportStateDiffFunc = exportStateDiff
	}
}

func WithRefundFunc(refund RefundFunc) Option {
	return func(opts *Options) {
		opts.refundFunc = refund
	}
}

func WithDimsModifierFunc(dimsModifier FeeManagerModifierFunc) Option {
	return func(opts *Options) {
		opts.dimsModifierFunc = dimsModifier
	}
}

func WithResultModifierFunc(resultModifier ResultModifierFunc) Option {
	return func(opts *Options) {
		opts.resultModifierFunc = resultModifier
	}
}

func applyOptions(option *Options, opts []Option) {
	for _, o := range opts {
		o(option)
	}
}
