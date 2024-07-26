import type { OnRpcRequestHandler } from '@metamask/snaps-sdk';
import { panel, text } from '@metamask/snaps-sdk';
import nacl from 'tweetnacl';
import { base58 } from '@scure/base';

/**
 * Handle incoming JSON-RPC requests, sent through `wallet_invokeSnap`.
 *
 * @param args - The request handler args as object.
 * @param args.origin - The origin of the request, e.g., the website that
 * invoked the snap.
 * @param args.request - A validated JSON-RPC request object.
 * @returns The result of `snap_dialog`.
 * @throws If the request method is not valid for this snap.
 */
export const onRpcRequest: OnRpcRequestHandler = async ({
  origin,
  request,
}) => {
  switch (request.method) {
    case 'hello':
      return snap.request({
        method: 'snap_dialog',
        params: {
          type: 'confirmation',
          content: panel([
            text(`Hello, **${origin}**!`),
            text('This custom confirmation is just for display purposes.'),
            text(
              'But you can edit the snap source code to make it do something, if you want to!',
            ),
          ]),
        },
      });
    case 'getPublicKey':
      const keyPair = await deriveKeyPairDefaultPath();

      const pubkey = base58.encode(keyPair.publicKey);

      return pubkey;
    default:
      throw new Error('Method not found.');
  }
};

import { SLIP10Node } from '@metamask/key-tree';

async function deriveKeyPairDefaultPath() {
  return deriveKeyPair(["0'"]);
}

async function deriveKeyPair(path: string[]) {
  assertIsArray(path);
  assertInput(path.length);
  assertInput(path.every((segment) => isValidSegment(segment)));

  const rootNode = await snap.request({
    method: 'snap_getBip32Entropy',
    params: {
      path: [`m`, `44'`, `9000'`],
      curve: 'ed25519'
    }
  });

  const node = await SLIP10Node.fromJSON(rootNode);

  const keypair = await node.derive(path.map((segment) => `slip10:${segment}`) as `slip10:${number}'`[]);
  if (!keypair.privateKeyBytes) {
    throw {
      code: -32000,
      message: 'error deriving key pair'
    };
  }

  return nacl.sign.keyPair.fromSeed(Uint8Array.from(keypair.privateKeyBytes));
}


export function assertIsArray(input: any[]) {
  if (!Array.isArray(input)) {
    throw {
      code: -32000,
      message: 'assertIsArray: Invalid input.'
    };
  }
}

function assertInput(path: any) {
  if (!path) {
    throw {
      code: -32000,
      message: 'assertInput: Invalid input.'
    };
  }
}


function isValidSegment(segment: string) {
  if (typeof segment !== 'string') {
    return false;
  }

  if (!segment.match(/^[0-9]+'$/)) {
    return false;
  }

  const index = segment.slice(0, -1);

  if (parseInt(index).toString() !== index) {
    return false;
  }

  return true;
}