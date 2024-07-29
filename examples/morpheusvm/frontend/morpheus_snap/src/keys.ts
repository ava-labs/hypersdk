export function isValidSegment(segment: string) {
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