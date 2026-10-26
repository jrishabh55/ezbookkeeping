import { describe, expect, test } from 'vitest';

import { clampBudgetProgressPercentage, getBudgetProgressColor, isValidBudgetThreshold } from '@/lib/budget.ts';

describe('clampBudgetProgressPercentage', () => {
    test('passes normal values through unchanged', () => {
        expect(clampBudgetProgressPercentage(0)).toBe(0);
        expect(clampBudgetProgressPercentage(42)).toBe(42);
        expect(clampBudgetProgressPercentage(100)).toBe(100);
    });

    test('clamps values above 100', () => {
        expect(clampBudgetProgressPercentage(150)).toBe(100);
    });

    test('clamps negative values to 0', () => {
        expect(clampBudgetProgressPercentage(-10)).toBe(0);
    });

    test('treats non-finite values as 0', () => {
        expect(clampBudgetProgressPercentage(NaN)).toBe(0);
        expect(clampBudgetProgressPercentage(Infinity)).toBe(0);
    });
});

describe('getBudgetProgressColor', () => {
    test('returns primary when no progress is available yet', () => {
        expect(getBudgetProgressColor(undefined)).toBe('primary');
    });

    test('returns primary when under every threshold', () => {
        expect(getBudgetProgressColor({ isOverspent: false, isAlert: false, isWarning: false })).toBe('primary');
    });

    test('returns orange once the warning threshold is crossed', () => {
        expect(getBudgetProgressColor({ isOverspent: false, isAlert: false, isWarning: true })).toBe('orange');
    });

    test('returns warning once the alert threshold is crossed, even if warning is also set', () => {
        expect(getBudgetProgressColor({ isOverspent: false, isAlert: true, isWarning: false })).toBe('warning');
        expect(getBudgetProgressColor({ isOverspent: false, isAlert: true, isWarning: true })).toBe('warning');
    });

    test('overspent takes priority over alert and warning', () => {
        expect(getBudgetProgressColor({ isOverspent: true, isAlert: true, isWarning: true })).toBe('error');
        expect(getBudgetProgressColor({ isOverspent: true, isAlert: false, isWarning: false })).toBe('error');
    });
});

describe('isValidBudgetThreshold', () => {
    test('accepts the inclusive range 0-100', () => {
        expect(isValidBudgetThreshold(0)).toBe(true);
        expect(isValidBudgetThreshold(100)).toBe(true);
        expect(isValidBudgetThreshold(75)).toBe(true);
    });

    test('rejects values outside the range', () => {
        expect(isValidBudgetThreshold(-1)).toBe(false);
        expect(isValidBudgetThreshold(101)).toBe(false);
    });

    test('rejects non-finite values', () => {
        expect(isValidBudgetThreshold(NaN)).toBe(false);
    });
});
