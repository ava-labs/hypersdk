


export function assertConfirmation(condition: boolean) {
    if (!condition) {
        throw {
            code: -32000,
            message: 'User rejected the request.'
        };
    }
}


export function assertIsString(input: any): asserts input is string {
    if (typeof input !== 'string') {
        throw {
            code: -32000,
            message: 'assertIsString: Input must be a string.'
        };
    }
}
export function assertInput(path: any) {
    if (!path) {
        throw {
            code: -32000,
            message: 'assertInput: Invalid input.'
        };
    }
}

export function assertIsArray(input: any[]) {
    if (!Array.isArray(input)) {
        throw {
            code: -32000,
            message: 'assertIsArray: Invalid input.'
        };
    }
}
