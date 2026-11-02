import {
    type ForecastResponse,
    type ForecastItemResponse,
    type ForecastShortfallResponse,
    ForecastItemType
} from '@/models/forecast.ts';

export interface ForecastSummary {
    closingCash: number;
    closingCashWithoutUncertain: number;
    expectedIncome: number;   // sum of uncertain income items this month
    shortfalls: ForecastShortfallResponse[];
    skippedCards: string[];
    nextItems: ForecastItemResponse[]; // next 5 items from today
}

const NEXT_ITEMS_COUNT: number = 5;

// summarise reduces a full forecast response down to the figures the overview widget needs:
// this month's closing cash (with and without uncertain income), the uncertain income total,
// the shortfall warnings, the skipped cards note, and the next few upcoming items from today.
export function summarise(resp: ForecastResponse, today: string): ForecastSummary {
    const thisMonth = resp.months[0];

    const closingCash = thisMonth ? thisMonth.closingCash : 0;
    const closingCashWithoutUncertain = thisMonth ? thisMonth.closingCashWithoutUncertain : 0;

    let expectedIncome = 0;

    for (const item of resp.items) {
        if (item.uncertain && item.type === ForecastItemType.Income) {
            expectedIncome += item.amount;
        }
    }

    const nextItems = resp.items
        .filter(item => item.date >= today)
        .sort((a, b) => a.date < b.date ? -1 : a.date > b.date ? 1 : 0)
        .slice(0, NEXT_ITEMS_COUNT);

    return {
        closingCash: closingCash,
        closingCashWithoutUncertain: closingCashWithoutUncertain,
        expectedIncome: expectedIncome,
        shortfalls: resp.shortfalls,
        skippedCards: resp.skippedCards,
        nextItems: nextItems
    };
}
