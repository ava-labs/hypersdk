import { copyable, divider, heading, panel, text } from '@metamask/snaps-sdk';

export function renderSignBytes(bytesBase58: string) {
    return snap.request({
        method: 'snap_dialog',
        params: {
            type: 'confirmation',
            content: panel([
                heading('Blind signing'),
                divider(),
                text("You are signing:"),
                copyable(bytesBase58)
            ])
        }
    });
}