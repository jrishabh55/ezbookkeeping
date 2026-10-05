export const BudgetPeriodType = {
    Custom: { type: 1, name: 'Custom' },
    Weekly: { type: 2, name: 'Weekly' },
    Monthly: { type: 3, name: 'Monthly' },
    Yearly: { type: 4, name: 'Yearly' }
} as const;

export const BudgetRolloverType = {
    Reset: { type: 1, name: 'Reset Every Cycle' },
    Rollover: { type: 2, name: 'Rollover Unused Amount' }
} as const;

export interface BudgetCycleInfoResponse {
    id: string;
    budgetId: string;
    startTime: number;
    endTime: number;
    carryoverAmount: number;
}

export interface BudgetInfoResponse {
    id: string;
    name: string;
    amount: number;
    currency: string;
    periodType: number;
    startTime: number;
    endTime: number;
    rolloverType: number;
    autoCreateNextCycle: boolean;
    warningThreshold: number;
    alertThreshold: number;
    overspentThreshold: number;
    displayOrder: number;
    hidden: boolean;
    categoryIds?: string[];
    accountIds?: string[];
    tagIds?: string[];
    currentCycle?: BudgetCycleInfoResponse;
}

export interface BudgetProgressResponse {
    budgetId: string;
    budgetName: string;
    budgetAmount: number;
    currency: string;
    spentAmount: number;
    remainingAmount: number;
    carryoverAmount: number;
    totalBudgetAmount: number;
    progressPercentage: number;
    startTime: number;
    endTime: number;
    warningThreshold: number;
    alertThreshold: number;
    overspentThreshold: number;
    isOverspent: boolean;
    isWarning: boolean;
    isAlert: boolean;
}

export class Budget implements BudgetInfoResponse {
    public id: string;
    public name: string;
    public amount: number;
    public currency: string;
    public periodType: number;
    public startTime: number;
    public endTime: number;
    public rolloverType: number;
    public autoCreateNextCycle: boolean;
    public warningThreshold: number;
    public alertThreshold: number;
    public overspentThreshold: number;
    public displayOrder: number;
    public hidden: boolean;
    public categoryIds?: string[];
    public accountIds?: string[];
    public tagIds?: string[];
    public currentCycle?: BudgetCycleInfoResponse;

    constructor(
        id: string,
        name: string,
        amount: number,
        currency: string,
        periodType: number,
        startTime: number,
        endTime: number,
        rolloverType: number,
        autoCreateNextCycle: boolean,
        warningThreshold: number,
        alertThreshold: number,
        overspentThreshold: number,
        displayOrder: number,
        hidden: boolean,
        categoryIds?: string[],
        accountIds?: string[],
        tagIds?: string[],
        currentCycle?: BudgetCycleInfoResponse
    ) {
        this.id = id;
        this.name = name;
        this.amount = amount;
        this.currency = currency;
        this.periodType = periodType;
        this.startTime = startTime;
        this.endTime = endTime;
        this.rolloverType = rolloverType;
        this.autoCreateNextCycle = autoCreateNextCycle;
        this.warningThreshold = warningThreshold;
        this.alertThreshold = alertThreshold;
        this.overspentThreshold = overspentThreshold;
        this.displayOrder = displayOrder;
        this.hidden = hidden;
        this.categoryIds = categoryIds;
        this.accountIds = accountIds;
        this.tagIds = tagIds;
        this.currentCycle = currentCycle;
    }

    public static createNewBudget(): Budget {
        return new Budget(
            '',
            '',
            0,
            '',
            BudgetPeriodType.Monthly.type,
            0,
            0,
            BudgetRolloverType.Reset.type,
            true,
            75,
            90,
            100,
            0,
            false,
            [],
            [],
            []
        );
    }

    public static fromResponse(response: BudgetInfoResponse): Budget {
        return new Budget(
            response.id,
            response.name,
            response.amount,
            response.currency,
            response.periodType,
            response.startTime,
            response.endTime,
            response.rolloverType,
            response.autoCreateNextCycle,
            response.warningThreshold,
            response.alertThreshold,
            response.overspentThreshold,
            response.displayOrder,
            response.hidden,
            response.categoryIds,
            response.accountIds,
            response.tagIds,
            response.currentCycle
        );
    }

    public getPeriodTypeName(): string {
        for (const key in BudgetPeriodType) {
            if (BudgetPeriodType[key as keyof typeof BudgetPeriodType].type === this.periodType) {
                return BudgetPeriodType[key as keyof typeof BudgetPeriodType].name;
            }
        }
        return 'Unknown';
    }

    public getRolloverTypeName(): string {
        for (const key in BudgetRolloverType) {
            if (BudgetRolloverType[key as keyof typeof BudgetRolloverType].type === this.rolloverType) {
                return BudgetRolloverType[key as keyof typeof BudgetRolloverType].name;
            }
        }
        return 'Unknown';
    }
}

export interface BudgetCreateRequest {
    name: string;
    amount: number;
    currency: string;
    periodType: number;
    startTime: number;
    endTime: number;
    rolloverType: number;
    autoCreateNextCycle: boolean;
    warningThreshold: number;
    alertThreshold: number;
    overspentThreshold: number;
    categoryIds?: string[];
    accountIds?: string[];
    tagIds?: string[];
    clientSessionId?: string;
}

export interface BudgetModifyRequest {
    id: string;
    name: string;
    amount: number;
    currency: string;
    periodType: number;
    startTime: number;
    endTime: number;
    rolloverType: number;
    autoCreateNextCycle: boolean;
    warningThreshold: number;
    alertThreshold: number;
    overspentThreshold: number;
    categoryIds?: string[];
    accountIds?: string[];
    tagIds?: string[];
    visible: boolean;
}

export interface BudgetHideRequest {
    id: string;
    hidden: boolean;
}

export interface BudgetNewDisplayOrderRequest {
    id: string;
    displayOrder: number;
}

export interface BudgetMoveRequest {
    newDisplayOrders: BudgetNewDisplayOrderRequest[];
}

export interface BudgetDeleteRequest {
    id: string;
}

export interface BudgetProgressRequest {
    id: string;
    startTime?: number;
    endTime?: number;
}
