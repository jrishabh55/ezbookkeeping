import type { BudgetProgressResponse } from '@/models/budget.ts';

// matches the 4 states the pre-refactor desktop budget list page distinguished
export type BudgetProgressColor = 'primary' | 'orange' | 'warning' | 'error';

// the server can return a percentage outside 0-100 (e.g. once a budget is overspent), clamp it for display
export function clampBudgetProgressPercentage(percentage: number): number {
    if (!Number.isFinite(percentage)) {
        return 0;
    }

    return Math.min(100, Math.max(0, percentage));
}

// overspent takes priority over alert, which takes priority over warning, which takes priority over no threshold crossed
export function getBudgetProgressColor(progress: Pick<BudgetProgressResponse, 'isOverspent' | 'isAlert' | 'isWarning'> | undefined): BudgetProgressColor {
    if (!progress) {
        return 'primary';
    }

    if (progress.isOverspent) {
        return 'error';
    }

    if (progress.isAlert) {
        return 'warning';
    }

    if (progress.isWarning) {
        return 'orange';
    }

    return 'primary';
}

export function isValidBudgetThreshold(threshold: number): boolean {
    return Number.isFinite(threshold) && threshold >= 0 && threshold <= 100;
}
