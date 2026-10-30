import { describe, expect, test } from 'vitest';

import {
    type EstimateStatus,
    ESTIMATE_DISPLAY_STATUS_NAMES,
    getEstimateDisplayStatus,
    isEstimateClosed,
    isEstimateLocked,
    canAnswerEstimate,
    canReviseEstimate,
    canDeleteEstimate,
    deletesEstimateChain,
    getDeleteEstimateConfirmation,
    getEstimateRevisionLabel,
    getSupersedingRevision,
    getMilestoneAmounts,
    getMilestonePercentTotal,
    getMilestonesProblem,
    addMilestonePercent,
    formatMilestonePercent,
    unixTimeToTextualDate
} from '@/lib/estimate.ts';
import { type EstimateRevision, Invoice } from '@/models/invoice.ts';
import type { TextualYearMonthDay } from '@/core/datetime.ts';

const estimate = (estimateStatus: '' | EstimateStatus, dueDate: TextualYearMonthDay, fullyInvoiced = false) =>
    ({ kind: 'estimate', estimateStatus, dueDate, fullyInvoiced, revisionNumber: 0, convertedInvoices: [] as { id: string, number: string }[] });

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
    test('edited in place only while a draft with nothing invoiced', () => {
        expect(isEstimateLocked(estimate('draft', '2099-01-01'))).toBe(false);
        expect(isEstimateLocked(estimate('', '2099-01-01'))).toBe(false);
        expect(isEstimateLocked(estimate('sent', '2099-01-01'))).toBe(true);
        expect(isEstimateLocked(estimate('accepted', '2099-01-01'))).toBe(true);
        expect(isEstimateLocked(estimate('superseded', '2099-01-01'))).toBe(true);
        expect(isEstimateLocked({ ...estimate('draft', '2099-01-01'), convertedInvoices: [{ id: '1', number: 'INV/2026-27/01' }] })).toBe(true);
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

describe('estimate revisions', () => {
    test('a superseded version shows as superseded, whatever its dates, and is closed', () => {
        expect(getEstimateDisplayStatus(estimate('superseded', '2026-01-01'), '2026-10-30')).toBe('superseded');
        expect(getEstimateDisplayStatus(estimate('superseded', '2099-01-01'), '2026-10-30')).toBe('superseded');
        expect(ESTIMATE_DISPLAY_STATUS_NAMES.superseded).toBe('Superseded');
        expect(isEstimateClosed(estimate('superseded', '2099-01-01'))).toBe(true);
        expect(canAnswerEstimate(estimate('superseded', '2099-01-01'), '2026-10-30')).toBe(false);
    });

    test('Revise is offered on sent (also expired), declined and accepted-not-invoiced versions', () => {
        expect(canReviseEstimate(estimate('sent', '2099-01-01'))).toBe(true);
        expect(canReviseEstimate(estimate('sent', '2026-01-01'))).toBe(true);
        expect(canReviseEstimate(estimate('declined', '2099-01-01'))).toBe(true);
        expect(canReviseEstimate(estimate('accepted', '2099-01-01'))).toBe(true);
        expect(canReviseEstimate({ ...estimate('accepted', '2099-01-01'), convertedInvoices: [{ id: '1', number: 'INV/2026-27/01' }] })).toBe(false);
        expect(canReviseEstimate(estimate('draft', '2099-01-01'))).toBe(false);
        expect(canReviseEstimate(estimate('superseded', '2099-01-01'))).toBe(false);
    });

    test('only the latest version with nothing invoiced can be deleted', () => {
        expect(canDeleteEstimate({ estimateStatus: 'sent', revisionNumber: 0 })).toBe(true);
        expect(canDeleteEstimate({ estimateStatus: 'draft', revisionNumber: 2 })).toBe(true);
        expect(canDeleteEstimate({ estimateStatus: 'sent', revisionNumber: 2 })).toBe(true);
        expect(canDeleteEstimate({ estimateStatus: 'accepted', revisionNumber: 1, convertedInvoices: [] })).toBe(true);
        expect(canDeleteEstimate({ estimateStatus: 'accepted', revisionNumber: 1, convertedInvoices: [{ id: '1', number: 'INV/2026-27/01' }] })).toBe(false);
        expect(canDeleteEstimate({ estimateStatus: 'superseded', revisionNumber: 0 })).toBe(false);
        expect(canDeleteEstimate({ estimateStatus: 'superseded', revisionNumber: 1 })).toBe(false);
    });

    test('deleting a revision that is no longer a draft deletes the whole chain', () => {
        expect(deletesEstimateChain({ estimateStatus: 'sent', revisionNumber: 1 })).toBe(true);
        expect(deletesEstimateChain({ estimateStatus: 'declined', revisionNumber: 2 })).toBe(true);
        expect(deletesEstimateChain({ estimateStatus: 'draft', revisionNumber: 1 })).toBe(false);
        expect(deletesEstimateChain({ estimateStatus: 'sent', revisionNumber: 0 })).toBe(false);
        expect(getDeleteEstimateConfirmation({ estimateStatus: 'accepted', revisionNumber: 1 })).toBe('Delete this estimate and all its revisions?');
        expect(getDeleteEstimateConfirmation({ estimateStatus: 'draft', revisionNumber: 1 })).toBe('Are you sure you want to delete this estimate?');
    });

    test('revision labels', () => {
        expect(getEstimateRevisionLabel(0)).toEqual({ key: 'Original', params: { number: 0 } });
        expect(getEstimateRevisionLabel(2)).toEqual({ key: 'format.misc.estimateRevision', params: { number: 2 } });
    });

    test('a version is superseded by the next version still in the chain', () => {
        const revisions = [{ id: 'c', revisionNumber: 3 }, { id: 'a', revisionNumber: 0 }]; // R1 and R2 were deleted drafts
        expect(getSupersedingRevision(revisions, 0)?.id).toBe('c');
        expect(getSupersedingRevision(revisions, 3)).toBeNull();
        expect(getSupersedingRevision([], 0)).toBeNull();
    });

    test('the detail response carries the revision number and every version of the chain', () => {
        const revisions: EstimateRevision[] = [
            { id: '9', number: 'EST/2026-27/01-R1', revisionNumber: 1, invoiceDate: '2026-10-30', dueDate: '2026-11-29', estimateStatus: 'draft', total: 100, currency: 'INR', fullyInvoiced: false },
            { id: '7', number: 'EST/2026-27/01', revisionNumber: 0, invoiceDate: '2026-04-23', dueDate: '2099-12-31', estimateStatus: 'superseded', total: 100, currency: 'INR', fullyInvoiced: false }
        ];
        const old = Invoice.of({ ...Invoice.createNewEstimate(), id: '7', estimateStatus: 'superseded', revisions });

        expect(old.revisionNumber).toBe(0);
        expect(old.revisions.map(revision => revision.number)).toEqual(['EST/2026-27/01-R1', 'EST/2026-27/01']);
        expect(getSupersedingRevision(old.revisions, old.revisionNumber)?.number).toBe('EST/2026-27/01-R1');
        expect(old.clone().revisions).toEqual(revisions);
        expect(old.duplicate().revisions).toEqual([]); // a duplicate is a new first version
        expect(Invoice.of({ ...Invoice.createNewEstimate(), id: '9', revisionNumber: 1 }).revisionNumber).toBe(1);
    });
});
