import type { RecurringInvoiceMode } from '@/models/invoice.ts';

export interface ProductInfoResponse {
    readonly id: string;
    readonly name: string;
    readonly description: string;
    readonly sac: string;
    readonly defaultRate: number;
    readonly currency: string; // '' = the profile's default currency
    readonly renewalMonths: number; // 0 = one-off
    readonly defaultRenewalMode: RecurringInvoiceMode | '';
    readonly archived: boolean;
}

export type ProductCreateRequest = Omit<ProductInfoResponse, 'id'>;

export interface ProductModifyRequest extends ProductCreateRequest {
    readonly id: string;
}

// unlinked invoice lines sharing an item name (trimmed, case-insensitive)
export interface InvoiceLineGroup {
    readonly itemName: string;
    readonly count: number;
}

export class Product implements ProductInfoResponse {
    public id: string = '';
    public name: string = '';
    public description: string = '';
    public sac: string = '';
    public defaultRate: number = 0;
    public currency: string = '';
    public renewalMonths: number = 0;
    public defaultRenewalMode: RecurringInvoiceMode | '' = '';
    public archived: boolean = false;

    public get renewable(): boolean {
        return this.renewalMonths > 0;
    }

    public fillFrom(other: ProductInfoResponse): void {
        this.id = other.id;
        this.name = other.name;
        this.description = other.description;
        this.sac = other.sac;
        this.defaultRate = other.defaultRate;
        this.currency = other.currency;
        this.renewalMonths = other.renewalMonths;
        this.defaultRenewalMode = other.defaultRenewalMode;
        this.archived = other.archived;
    }

    public toCreateRequest(): ProductCreateRequest {
        return {
            name: this.name.trim(),
            description: this.description,
            sac: this.sac.trim(),
            defaultRate: this.defaultRate,
            currency: this.currency,
            renewalMonths: this.renewalMonths,
            defaultRenewalMode: this.renewalMonths > 0 ? (this.defaultRenewalMode || 'draft_ahead') : '',
            archived: this.archived
        };
    }

    public toModifyRequest(): ProductModifyRequest {
        return { id: this.id, ...this.toCreateRequest() };
    }

    public clone(): Product {
        return Product.of(this);
    }

    public static of(response: ProductInfoResponse): Product {
        const product = new Product();
        product.fillFrom(response);
        return product;
    }

    public static ofMulti(responses: ProductInfoResponse[]): Product[] {
        return responses.map(response => Product.of(response));
    }

    public static createNewProduct(): Product {
        return new Product();
    }
}
