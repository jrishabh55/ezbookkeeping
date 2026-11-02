// Mirrors pkg/models/forecast.go ForecastResponse and friends

export enum ForecastItemType {
    Expense = 1,
    Income = 2,
    Transfer = 3
}

export type ForecastItemSource = 'template' | 'invoice' | 'recurring_invoice' | 'everyday' | 'card_bill';

export interface ForecastMonthResponse {
    month: string;
    openingCash: number;
    closingCash: number;
    closingCashWithoutUncertain: number;
    businessIncome: number;
    businessCost: number;
    personalCost: number;
}

export interface ForecastItemResponse {
    date: string;
    type: ForecastItemType;
    accountId: string;
    destinationAccountId?: string;
    categoryId?: string;
    amount: number;
    source: ForecastItemSource;
    sourceId?: string;
    uncertain: boolean;
    business: boolean;
    label: string;
}

export interface ForecastShortfallResponse {
    accountId: string;
    date: string;
    lowestBalance: number;
    lowestDate: string;
}

export interface ForecastResponse {
    months: ForecastMonthResponse[];
    items: ForecastItemResponse[];
    shortfalls: ForecastShortfallResponse[];
    skippedCards: string[];
}
