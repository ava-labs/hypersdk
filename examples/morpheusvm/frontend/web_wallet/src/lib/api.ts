import { base64 } from '@scure/base';

const API_BASE_URL = 'http://localhost:9650/ext/bc/morpheusvm';

interface ApiResponse<T> {
    result: T;
    error?: {
        message: string;
    };
}

async function _makeApiRequest<T>(namespace: string, method: string, params: object = {}): Promise<T> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 3000);

    try {
        const response = await fetch(`${API_BASE_URL}/${namespace}`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify({
                jsonrpc: "2.0",
                method,
                params,
                id: parseInt(String(Math.random()).slice(2))
            }),
            signal: controller.signal
        });

        const json: ApiResponse<T> = await response.json();
        if (json?.error?.message) {
            throw new Error(json.error.message);
        }
        return json.result;
    } catch (error: any) {
        if (error.name === 'AbortError') {
            throw new Error('Request timed out after 3 seconds');
        }
        throw error;
    } finally {
        clearTimeout(timeoutId);
    }
}

export async function getBalance(address: string): Promise<bigint> {
    const result = await _makeApiRequest<{ amount: string }>("morpheusapi", 'morpheusvm.balance', { address });
    return BigInt(result.amount);
}

export async function getNetwork(): Promise<{ networkId: number, subnetId: string, chainId: string }> {
    return _makeApiRequest<{ networkId: number, subnetId: string, chainId: string }>("coreapi", 'hypersdk.network');
}

export async function sendTx(txBytes: Uint8Array): Promise<void> {
    const bytesBase64 = base64.encode(txBytes);
    await _makeApiRequest<void>("coreapi", 'hypersdk.submitTx', { tx: bytesBase64 });
}
