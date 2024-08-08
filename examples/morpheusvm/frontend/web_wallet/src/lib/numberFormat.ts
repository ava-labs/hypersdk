
export const fromFormattedBalance = (balance: string, decimals: number = 9): bigint => {
    const float = parseFloat(balance)
    return BigInt(float * 10 ** decimals)
}

export const formatBalance = (balance: bigint, decimals: number = 9): string => {
    //TODO: refactor
    const divisor = 10n ** BigInt(decimals);
    const quotient = balance / divisor;
    const remainder = balance % divisor;
    const paddedRemainder = remainder.toString().padStart(decimals, '0');
    return `${quotient}.${paddedRemainder}`;
}
