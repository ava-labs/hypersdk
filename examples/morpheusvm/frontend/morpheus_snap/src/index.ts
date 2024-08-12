import { SLIP10Node } from '@metamask/key-tree';
import type { OnRpcRequestHandler } from '@metamask/snaps-sdk';
import { base58 } from '@scure/base';
import nacl from 'tweetnacl';
import { assertInput, assertIsString, assertConfirmation, assertIsArray } from './assert';
import { isValidSegment } from './keys';
import { renderSignBytes } from './ui';
import { Marshaler } from './Marshaler';
import { signTransactionBytes, TransactionPayload } from './sign';



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
  let keyPair: nacl.SignKeyPair;

 /* if (request.method === 'signBytes') {
    // const dappHost = (new URL(origin))?.host;

    const { derivationPath, bytesBase58 } = (request.params || {}) as { derivationPath?: string[], bytesBase58?: string };

    assertInput(bytesBase58);
    const bytes = base58.decode(bytesBase58 || "");
    assertInput(bytes.length);

    keyPair = await deriveKeyPair(derivationPath || []);

    const accepted = await renderSignBytes(bytesBase58 || "");
    assertConfirmation(!!accepted);

    const signature = nacl.sign.detached(bytes, keyPair.secretKey);

    return base58.encode(signature)
  } else */if (request.method === 'signTransaction') {
    const { derivationPath, tx, abiString } = (request.params || {}) as { derivationPath?: string[], tx: TransactionPayload, abiString: string };

    keyPair = await deriveKeyPair(derivationPath || []);

    const marshaler = new Marshaler(abiString);
    const digest = marshaler.encodeTransaction(tx);

    const accepted = await renderSignBytes(tx);
    assertConfirmation(!!accepted);

    const signedTxBytes = signTransactionBytes(digest, keyPair.secretKey.slice(0, 32));

    return base58.encode(signedTxBytes);
  } else if (request.method === 'getPublicKey') {
    const { derivationPath } = (request.params || {}) as { derivationPath?: string[] };

    keyPair = await deriveKeyPair(derivationPath || []);

    const pubkey = base58.encode(keyPair.publicKey);

    return pubkey;
  } else {
    throw new Error('Method not found.');
  }
};

async function deriveKeyPair(path: string[]): Promise<nacl.SignKeyPair> {
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
