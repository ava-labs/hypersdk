// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context, ContractId};

#[public]
pub fn deploy(ctx: &mut Context, contract_id: ContractId) -> Address {
    ctx.deploy(contract_id, &[])
}
