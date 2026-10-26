import type { BudgetProgressResponse } from '@/models/budget.ts';

export type BudgetProgressColor = 'success' | 'warning' | 'error';

// the server can return a percentage outside 0-100 (e.g. once a budget is overspent), clamp it for display
export function clampBudgetProgressPercentage(percentage: number): number {
    if (!Number.isFinite(percentage)) {
        return 0;
    }

    return Math.min(100, Math.max(0, percentage));
}

// overspent takes priority over alert, which takes priority over warning
export function getBudgetProgressColor(progress: Pick<BudgetProgressResponse, 'isOverspent' | 'isAlert' | 'isWarning'> | undefined): BudgetProgressColor {
    if (!progress) {
        return 'success';
    }

    if (progress.isOverspent) {
        return 'error';
    }

    if (progress.isAlert || progress.isWarning) {
        return 'warning';
    }

    return 'success';
}

export function isValidBudgetThreshold(threshold: number): boolean {
    return Number.isFinite(threshold) && threshold >= 0 && threshold <= 100;
}
