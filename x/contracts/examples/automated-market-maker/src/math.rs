// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// https://ethereum.stackexchange.com/questions/2910/can-i-square-root-in-solidity
// https://github.com/Uniswap/v2-core/blob/v1.0.1/contracts/libraries/Math.sol
pub(crate) fn sqrt(y: u64) -> u64 {
    if y > 3 {
        let mut z = y;
        let mut x = y / 2 + 1;
        while x < z {
            z = x;
            x = (y / x + x) / 2;
        }
        z
    } else if y > 0 {
        1
    } else {
        0
    }
}
