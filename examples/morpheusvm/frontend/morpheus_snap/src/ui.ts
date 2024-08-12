import { copyable, divider, heading, panel, text } from '@metamask/snaps-sdk';
import { TransactionPayload } from './sign';

export function renderSignBytes(tx: TransactionPayload) {

    return snap.request({
        method: 'snap_dialog',
        params: {
            type: 'confirmation',
            content: panel([
                heading('Sign Transaction'),
                divider(),
                ...tx.actions.map((action) => {
                    return [
                        heading(action.actionName),

                        ...Object.entries(action.data).map(([key, value]) => {
                            return [
                                text(key + ":"),
                                copyable(JSON.stringify(value, null, 2)),
                                divider(),
                            ];
                        }).flat(),

                    ];
                }).flat(),

                text("Warning: This JSON might have extra fields, field validation is not implemented yet.")
            ])
        }
    });
}