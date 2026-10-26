import { describe, expect, test } from 'vitest';

import { clampBucketProgressPercentage, isValidBucketColor } from '@/lib/bucket.ts';

describe('clampBucketProgressPercentage', () => {
    test('passes normal values through unchanged', () => {
        expect(clampBucketProgressPercentage(0)).toBe(0);
        expect(clampBucketProgressPercentage(55)).toBe(55);
        expect(clampBucketProgressPercentage(100)).toBe(100);
    });

    test('clamps values above 100 once a bucket is over-funded', () => {
        expect(clampBucketProgressPercentage(130)).toBe(100);
    });

    test('clamps negative values to 0', () => {
        expect(clampBucketProgressPercentage(-5)).toBe(0);
    });

    test('treats non-finite values as 0', () => {
        expect(clampBucketProgressPercentage(NaN)).toBe(0);
        expect(clampBucketProgressPercentage(Infinity)).toBe(0);
    });
});

describe('isValidBucketColor', () => {
    test('accepts a 6 digit hex value', () => {
        expect(isValidBucketColor('4caf50')).toBe(true);
        expect(isValidBucketColor('FFFFFF')).toBe(true);
    });

    test('rejects blank, short or non-hex values', () => {
        expect(isValidBucketColor('')).toBe(false);
        expect(isValidBucketColor('fff')).toBe(false);
        expect(isValidBucketColor('zzzzzz')).toBe(false);
    });
});
