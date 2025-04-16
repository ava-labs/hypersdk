// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package context provides a generic map-based configuration for storing
// multiple configurations in a single value.
//
// As part of the [common.VM] interface, the Initialize method takes in a
// config. With the introduction of the snow package, this config is now
// shared between multiple packages, including the snow and vm packages.
// [context] helps manage this shared config by allowing packages to define
// their own sub-configurations within the main config.
//
// [common.VM]: https://github.com/ava-labs/avalanchego/blob/a353b45944904ce56b5eddf961bf83430892dc32/snow/engine/common/vm.go#L17
package context
