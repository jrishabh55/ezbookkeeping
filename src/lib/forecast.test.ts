import { describe, expect, it } from 'vitest';

import { summarise } from './forecast.ts';
import type { ForecastResponse } from '@/models/forecast.ts';

const resp: ForecastResponse = {
    months: [{ month: '2026-11', openingCash: 500, closingCash: 1350, closingCashWithoutUncertain: 350, businessIncome: 1000, businessCost: 0, personalCost: 100 }],
    items: [
        { date: '2026-11-01', type: 1, accountId: '1', amount: 1, source: 'template', uncertain: false, business: false, label: 'past' },
        { date: '2026-11-05', type: 1, accountId: '1', amount: 100, source: 'template', uncertain: false, business: false, label: 'Car EMI' },
        { date: '2026-11-28', type: 2, accountId: '1', amount: 1000, source: 'recurring_invoice', uncertain: true, business: true, label: 'Quber' }
    ],
    shortfalls: [{ accountId: '2', date: '2026-11-05', lowestBalance: -1300, lowestDate: '2026-11-15' }],
    skippedCards: ['9']
};

describe('summarise', () => {
    it('takes this month, uncertain income and upcoming items', () => {
        const s = summarise(resp, '2026-11-02');
        expect(s.closingCash).toBe(1350);
        expect(s.closingCashWithoutUncertain).toBe(350);
        expect(s.expectedIncome).toBe(1000);
        expect(s.nextItems.map(i => i.label)).toEqual(['Car EMI', 'Quber']);
        expect(s.shortfalls).toHaveLength(1);
        expect(s.skippedCards).toEqual(['9']);
    });
});
