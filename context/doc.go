// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package context provides a generic map-based configuration for storing
// multiple configurations in a single value.
//
// The config passed in during VM initialization is used by several packages,
// including the snow and vm packages. The [context] package helps manage this
// by allowing packages to define their own sub-configurations within the main config.
//
// [common.VM]: https://github.com/ava-labs/avalanchego/blob/a353b45944904ce56b5eddf961bf83430892dc32/snow/engine/common/vm.go#L17
package context
