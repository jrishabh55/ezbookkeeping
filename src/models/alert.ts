export interface AlertTokenCreateResponse {
    readonly token: string;
    readonly shortcutUrl: string;
}

export interface AlertStatusResponse {
    readonly configured: boolean;
    readonly lastReceivedAt: number;
    readonly lastOutcome: string;
    readonly counts: Record<string, number>;
    readonly shortcutUrl: string;
}
