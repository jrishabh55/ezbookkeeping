// the server derives progress from currentBalance / targetAmount, clamp it for display
export function clampBucketProgressPercentage(percentage: number): number {
    if (!Number.isFinite(percentage)) {
        return 0;
    }

    return Math.min(100, Math.max(0, percentage));
}

export function isValidBucketColor(color: string): boolean {
    return /^[0-9a-fA-F]{6}$/.test(color);
}
