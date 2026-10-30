import { describe, expect, test } from 'vitest';

import {
    getEstimateDisplayStatus,
    isEstimateClosed,
    isEstimateLocked,
    canAnswerEstimate,
    getMilestoneAmounts,
    getMilestonePercentTotal,
    getMilestonesProblem,
    addMilestonePercent,
    formatMilestonePercent,
    unixTimeToTextualDate
} from '@/lib/estimate.ts';
import { Invoice } from '@/models/invoice.ts';
import type { TextualYearMonthDay } from '@/core/datetime.ts';

const estimate = (estimateStatus: '' | 'draft' | 'sent' | 'accepted' | 'declined', dueDate: TextualYearMonthDay, fullyInvoiced = false) =>
    ({ kind: 'estimate', estimateStatus, dueDate, fullyInvoiced, convertedInvoices: [] as { id: string, number: string }[] });

describe('getMilestoneAmounts', () => {
    test('rounds each share half up and gives the last one the remainder', () => {
        expect(getMilestoneAmounts(235999, [3333, 3333, 3334])).toEqual([78658, 78658, 78683]);
        expect(getMilestoneAmounts(5, [5000, 5000])).toEqual([3, 2]);
    });

    test('always adds up to the amount', () => {
        const amounts = getMilestoneAmounts(199999, [1000, 2500, 2500, 4000]);
        expect(amounts.reduce((sum, amount) => sum + amount, 0)).toBe(199999);
    });

    test('shows plain shares while the percents do not add up to 100% yet', () => {
        expect(getMilestoneAmounts(100000, [3000, 3000])).toEqual([30000, 30000]);
        expect(getMilestoneAmounts(100000, [])).toEqual([]);
    });
});

describe('getMilestonesProblem', () => {
    test('none, or 1 to 10 labelled milestones adding up to 100%', () => {
        expect(getMilestonesProblem([])).toBeNull();
        expect(getMilestonesProblem([{ label: 'Advance', percent: 3000 }, { label: 'Delivery', percent: 7000 }])).toBeNull();
        expect(getMilestonesProblem([{ label: ' ', percent: 10000 }])).toBe('Milestone label cannot be blank');
        expect(getMilestonesProblem([{ label: 'A'.repeat(129), percent: 10000 }])).toBe('Milestone label cannot be longer than 128 characters');
        expect(getMilestonesProblem([{ label: 'A'.repeat(128), percent: 10000 }])).toBeNull();
        expect(getMilestonesProblem([{ label: 'Advance', percent: 0 }, { label: 'Delivery', percent: 10000 }])).toBe('Milestone percent must be greater than zero');
        expect(getMilestonesProblem([{ label: 'Advance', percent: 5000 }, { label: 'Delivery', percent: 4999 }])).toBe('Milestones must add up to 100%');
        expect(getMilestonesProblem(Array.from({ length: 11 }, () => ({ label: 'Part', percent: 909 })))).toBe('An estimate can have at most 10 milestones');
        expect(getMilestonePercentTotal([{ percent: 3333 }, { percent: 3333 }])).toBe(6666);
        expect(formatMilestonePercent(3333)).toBe('33.33%');
    });
});

describe('addMilestonePercent', () => {
    test('splits the last milestone in half once the percents add up to 100%, otherwise gives the remainder', () => {
        expect(addMilestonePercent([])).toEqual([10000]);
        expect(addMilestonePercent([10000])).toEqual([5000, 5000]);
        expect(addMilestonePercent([5000, 5000])).toEqual([5000, 2500, 2500]);
        expect(addMilestonePercent([6000])).toEqual([6000, 4000]);
    });

    test('an odd split still sums to the old percent', () => {
        expect(addMilestonePercent([9999, 1])).toEqual([9999, 1, 0]); // 1 can only split 1 / 0
        expect(addMilestonePercent([3333, 3333, 3334])).toEqual([3333, 3333, 1667, 1667]);
    });
});

describe('getEstimateDisplayStatus', () => {
    test('expired is a draft or sent estimate past its valid-until date', () => {
        expect(getEstimateDisplayStatus(estimate('sent', '2026-10-29'), '2026-10-30')).toBe('expired');
        expect(getEstimateDisplayStatus(estimate('draft', '2026-10-29'), '2026-10-30')).toBe('expired');
        expect(getEstimateDisplayStatus(estimate('sent', '2026-10-30'), '2026-10-30')).toBe('sent'); // valid until today
        expect(getEstimateDisplayStatus(estimate('accepted', '2026-01-01'), '2026-10-30')).toBe('accepted');
        expect(getEstimateDisplayStatus(estimate('declined', '2026-01-01'), '2026-10-30')).toBe('declined');
        expect(getEstimateDisplayStatus(estimate('', '2099-01-01'), '2026-10-30')).toBe('draft');
    });

    test('a fully invoiced estimate shows as invoiced and is closed', () => {
        expect(getEstimateDisplayStatus(estimate('accepted', '2026-01-01', true), '2026-10-30')).toBe('invoiced');
        expect(isEstimateClosed(estimate('accepted', '2026-01-01', true))).toBe(true);
        expect(isEstimateClosed(estimate('declined', '2099-01-01'))).toBe(true);
        expect(isEstimateClosed(estimate('accepted', '2099-01-01'))).toBe(false);
    });
});

describe('isEstimateLocked and canAnswerEstimate', () => {
    test('editable while draft or sent and nothing invoiced', () => {
        expect(isEstimateLocked(estimate('sent', '2099-01-01'))).toBe(false);
        expect(isEstimateLocked(estimate('accepted', '2099-01-01'))).toBe(true);
        expect(isEstimateLocked({ ...estimate('sent', '2099-01-01'), convertedInvoices: [{ id: '1', number: 'INV/2026-27/01' }] })).toBe(true);
    });

    test('the client can answer only a sent, still valid estimate', () => {
        expect(canAnswerEstimate(estimate('sent', '2026-10-30'), '2026-10-30')).toBe(true);
        expect(canAnswerEstimate(estimate('sent', '2026-10-29'), '2026-10-30')).toBe(false);
        expect(canAnswerEstimate(estimate('draft', '2099-01-01'), '2026-10-30')).toBe(false);
        expect(canAnswerEstimate({ ...estimate('sent', '2099-01-01'), kind: 'invoice' }, '2026-10-30')).toBe(false);
    });
});

describe('estimate models', () => {
    test('duplicating an answered estimate makes a new draft with the same payment terms', () => {
        const source = Invoice.createNewEstimate();
        source.estimateStatus = 'accepted';
        source.milestones = [{ id: '7', label: 'Advance', percent: 3000, invoiceId: '9', invoiceNumber: 'INV/2026-27/01' }, Invoice.createNewMilestone(7000, 'Delivery')];

        const copy = source.duplicate();
        expect(copy.kind).toBe('estimate');
        expect(copy.estimateStatus).toBe('draft');
        expect(copy.milestones).toEqual([
            { id: '', label: 'Advance', percent: 3000, invoiceId: '', invoiceNumber: '' },
            { id: '', label: 'Delivery', percent: 7000, invoiceId: '', invoiceNumber: '' }
        ]);
        expect(copy.toCreateRequest().milestones).toEqual([{ label: 'Advance', percent: 3000 }, { label: 'Delivery', percent: 7000 }]);
        expect(copy.toCreateRequest().kind).toBe('estimate');
    });

    test('an invoice sends no milestones and no estimate link unless converted', () => {
        const request = Invoice.createNewInvoice().toCreateRequest();
        expect(request.kind).toBe('invoice');
        expect(request.milestones).toEqual([]);
        expect(request.estimateId).toBe('0');
        expect(request.milestoneId).toBe('0');
    });

    test('unixTimeToTextualDate is the local calendar date', () => {
        expect(unixTimeToTextualDate(new Date(2026, 9, 30, 12, 0, 0).getTime() / 1000)).toBe('2026-10-30');
    });
});
