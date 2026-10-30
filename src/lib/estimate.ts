import type { TextualYearMonthDay } from '@/core/datetime.ts';

// mirror models.EstimateMilestonePercentTotal / models.EstimateMaxMilestones / models.EstimateMilestoneLabelMaxLength in pkg/models/estimate.go
export const ESTIMATE_MILESTONE_PERCENT_TOTAL = 10000;
export const ESTIMATE_MAX_MILESTONES = 10;
export const ESTIMATE_MILESTONE_LABEL_MAX_LENGTH = 128;

// mirrors models.ESTIMATE_STATUS_*; expired and invoiced are only ever displayed
export type EstimateStatus = 'draft' | 'sent' | 'accepted' | 'declined';
export type EstimateDisplayStatus = EstimateStatus | 'expired' | 'invoiced';

export const ESTIMATE_DISPLAY_STATUS_NAMES: Record<EstimateDisplayStatus, string> = {
    draft: 'Draft',
    sent: 'Sent',
    accepted: 'Accepted',
    declined: 'Declined',
    expired: 'Expired',
    invoiced: 'Invoiced'
};

interface EstimateState {
    readonly estimateStatus: EstimateStatus | '';
    readonly dueDate: string;
    readonly fullyInvoiced: boolean;
}

// expired: a draft or sent estimate whose valid-until date (dueDate) is before today; extending the date revives it
export function getEstimateDisplayStatus(estimate: EstimateState, today: string): EstimateDisplayStatus {
    if (estimate.fullyInvoiced) {
        return 'invoiced';
    }

    const status = estimate.estimateStatus || 'draft';

    if ((status === 'draft' || status === 'sent') && estimate.dueDate < today) {
        return 'expired';
    }

    return status;
}

// hidden from the list unless "Show closed": declined, or fully invoiced
export function isEstimateClosed(estimate: EstimateState): boolean {
    return estimate.estimateStatus === 'declined' || estimate.fullyInvoiced;
}

// read-only once answered or once anything was invoiced from it, mirroring checkEstimateEditable in pkg/services/estimate_lifecycle.go
export function isEstimateLocked(estimate: { readonly estimateStatus: EstimateStatus | '', readonly convertedInvoices: readonly unknown[] }): boolean {
    const answered = estimate.estimateStatus === 'accepted' || estimate.estimateStatus === 'declined';
    return answered || estimate.convertedInvoices.length > 0;
}

// the public page offers Accept / Decline only while the server would take the answer
export function canAnswerEstimate(estimate: { readonly kind: string, readonly estimateStatus: EstimateStatus | '', readonly dueDate: string }, today: string): boolean {
    return estimate.kind === 'estimate' && estimate.estimateStatus === 'sent' && estimate.dueDate >= today;
}

// amount × percent (basis points) / 10000 rounded half up, mirroring milestoneShare in pkg/invoicepdf/format.go
export function getEstimateShare(amount: number, percent: number): number {
    return Math.round(amount * percent / ESTIMATE_MILESTONE_PERCENT_TOTAL);
}

// each milestone's part of amount (the total incl. tax in the Payment Terms table and the editor); once the percents add up to
// 100% the last one takes what is left, so the parts add up exactly
export function getMilestoneAmounts(amount: number, percents: readonly number[]): number[] {
    const amounts = percents.map(percent => getEstimateShare(amount, percent));

    if (amounts.length && percents.reduce((sum, percent) => sum + percent, 0) === ESTIMATE_MILESTONE_PERCENT_TOTAL) {
        amounts[amounts.length - 1] = amount - amounts.slice(0, -1).reduce((sum, value) => sum + value, 0);
    }

    return amounts;
}

export function getMilestonePercentTotal(milestones: readonly { readonly percent: number }[]): number {
    return milestones.reduce((sum, milestone) => sum + milestone.percent, 0);
}

// the percents (basis points) after adding one more milestone: while they already add up to 100%, the last milestone's
// percent is split in half (rounded so the two halves still sum to its old value) and the new milestone gets the other
// half; otherwise the new milestone gets whatever is still missing, so two clicks on a fresh estimate give a sensible 50 / 50
export function addMilestonePercent(existingPercents: readonly number[]): number[] {
    const percents = [...existingPercents];
    const total = getMilestonePercentTotal(percents.map(percent => ({ percent })));

    if (percents.length && total === ESTIMATE_MILESTONE_PERCENT_TOTAL) {
        const lastPercent = percents[percents.length - 1]!;
        const firstHalf = Math.round(lastPercent / 2);
        percents[percents.length - 1] = firstHalf;
        percents.push(lastPercent - firstHalf);
    } else {
        percents.push(Math.max(ESTIMATE_MILESTONE_PERCENT_TOTAL - total, 0));
    }

    return percents;
}

// mirrors validateEstimateMilestones in pkg/services/estimates.go; null when the milestones can be saved
export function getMilestonesProblem(milestones: readonly { readonly label: string, readonly percent: number }[]): string | null {
    if (!milestones.length) {
        return null;
    } else if (milestones.length > ESTIMATE_MAX_MILESTONES) {
        return 'An estimate can have at most 10 milestones';
    } else if (milestones.some(milestone => !milestone.label.trim())) {
        return 'Milestone label cannot be blank';
    } else if (milestones.some(milestone => milestone.label.trim().length > ESTIMATE_MILESTONE_LABEL_MAX_LENGTH)) {
        return 'Milestone label cannot be longer than 128 characters';
    } else if (milestones.some(milestone => milestone.percent <= 0)) {
        return 'Milestone percent must be greater than zero';
    } else if (getMilestonePercentTotal(milestones) !== ESTIMATE_MILESTONE_PERCENT_TOTAL) {
        return 'Milestones must add up to 100%';
    }

    return null;
}

// basis points as a percentage with trailing zeros trimmed: 3000 → "30%", 3333 → "33.33%"
export function formatMilestonePercent(basisPoints: number): string {
    return (basisPoints / 100).toString() + '%';
}

// the local calendar date of a unix time, e.g. for "Accepted on <date>"
export function unixTimeToTextualDate(unixTime: number): TextualYearMonthDay {
    const date = new Date(unixTime * 1000);
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}` as TextualYearMonthDay;
}
