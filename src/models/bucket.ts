import type { ColorValue } from '@/core/color.ts';

export const BucketStatus = {
    InProgress: { type: 1, name: 'In Progress' },
    Completed: { type: 2, name: 'Completed' }
} as const;

export const BucketTransactionType = {
    Allocate: { type: 1, name: 'Allocate' },
    Withdraw: { type: 2, name: 'Withdraw' }
} as const;

export interface BucketInfoResponse {
    id: string;
    name: string;
    targetAmount: number;
    currentBalance: number;
    currency: string;
    status: number;
    icon: string;
    color: ColorValue;
    comment: string;
    displayOrder: number;
    hidden: boolean;
    accountIds?: string[];
    progress: number;
}

export interface BucketTransactionInfoResponse {
    id: string;
    bucketId: string;
    type: number;
    amount: number;
    transactionId?: string;
    transactionTime: number;
    utcOffset: number;
    comment: string;
}

export class Bucket implements BucketInfoResponse {
    public id: string;
    public name: string;
    public targetAmount: number;
    public currentBalance: number;
    public currency: string;
    public status: number;
    public icon: string;
    public color: ColorValue;
    public comment: string;
    public displayOrder: number;
    public hidden: boolean;
    public accountIds?: string[];
    public progress: number;

    constructor(
        id: string,
        name: string,
        targetAmount: number,
        currentBalance: number,
        currency: string,
        status: number,
        icon: string,
        color: ColorValue,
        comment: string,
        displayOrder: number,
        hidden: boolean,
        progress: number,
        accountIds?: string[]
    ) {
        this.id = id;
        this.name = name;
        this.targetAmount = targetAmount;
        this.currentBalance = currentBalance;
        this.currency = currency;
        this.status = status;
        this.icon = icon;
        this.color = color;
        this.comment = comment;
        this.displayOrder = displayOrder;
        this.hidden = hidden;
        this.progress = progress;
        this.accountIds = accountIds;
    }

    public static createNewBucket(): Bucket {
        return new Bucket(
            '',
            '',
            0,
            0,
            '',
            BucketStatus.InProgress.type,
            '1',
            '000000',
            '',
            0,
            false,
            0,
            []
        );
    }

    public static fromResponse(response: BucketInfoResponse): Bucket {
        return new Bucket(
            response.id,
            response.name,
            response.targetAmount,
            response.currentBalance,
            response.currency,
            response.status,
            response.icon,
            response.color,
            response.comment,
            response.displayOrder,
            response.hidden,
            response.progress,
            response.accountIds
        );
    }

    public getStatusName(): string {
        for (const key in BucketStatus) {
            if (BucketStatus[key as keyof typeof BucketStatus].type === this.status) {
                return BucketStatus[key as keyof typeof BucketStatus].name;
            }
        }
        return 'Unknown';
    }

    public isCompleted(): boolean {
        return this.status === BucketStatus.Completed.type;
    }

    public isInProgress(): boolean {
        return this.status === BucketStatus.InProgress.type;
    }
}

export class BucketTransaction implements BucketTransactionInfoResponse {
    public id: string;
    public bucketId: string;
    public type: number;
    public amount: number;
    public transactionId?: string;
    public transactionTime: number;
    public utcOffset: number;
    public comment: string;

    constructor(
        id: string,
        bucketId: string,
        type: number,
        amount: number,
        transactionTime: number,
        utcOffset: number,
        comment: string,
        transactionId?: string
    ) {
        this.id = id;
        this.bucketId = bucketId;
        this.type = type;
        this.amount = amount;
        this.transactionTime = transactionTime;
        this.utcOffset = utcOffset;
        this.comment = comment;
        this.transactionId = transactionId;
    }

    public static fromResponse(response: BucketTransactionInfoResponse): BucketTransaction {
        return new BucketTransaction(
            response.id,
            response.bucketId,
            response.type,
            response.amount,
            response.transactionTime,
            response.utcOffset,
            response.comment,
            response.transactionId
        );
    }

    public isAllocate(): boolean {
        return this.type === BucketTransactionType.Allocate.type;
    }

    public isWithdraw(): boolean {
        return this.type === BucketTransactionType.Withdraw.type;
    }

    public getTypeName(): string {
        for (const key in BucketTransactionType) {
            if (BucketTransactionType[key as keyof typeof BucketTransactionType].type === this.type) {
                return BucketTransactionType[key as keyof typeof BucketTransactionType].name;
            }
        }
        return 'Unknown';
    }
}

export interface BucketCreateRequest {
    name: string;
    targetAmount: number;
    currency: string;
    icon: string;
    color: string;
    comment: string;
    accountIds?: string[];
    clientSessionId?: string;
}

export interface BucketModifyRequest {
    id: string;
    name: string;
    targetAmount: number;
    currency: string;
    icon: string;
    color: string;
    comment: string;
    status: number;
    accountIds?: string[];
    visible: boolean;
}

export interface BucketHideRequest {
    id: string;
    hidden: boolean;
}

export interface BucketNewDisplayOrderRequest {
    id: string;
    displayOrder: number;
}

export interface BucketMoveRequest {
    newDisplayOrders: BucketNewDisplayOrderRequest[];
}

export interface BucketDeleteRequest {
    id: string;
}

export interface BucketAllocateRequest {
    bucketId: string;
    amount: number;
    transactionId?: string;
    transactionTime: number;
    utcOffset: number;
    comment: string;
    clientSessionId?: string;
}

export interface BucketWithdrawRequest {
    bucketId: string;
    amount: number;
    transactionId?: string;
    transactionTime: number;
    utcOffset: number;
    comment: string;
    clientSessionId?: string;
}

export interface BucketTransactionListRequest {
    bucketId: string;
    maxTime?: number;
    minTime?: number;
    count?: number;
}

export interface BucketTransactionDeleteRequest {
    id: string;
}
