import type { TextualYearMonthDay } from '@/core/datetime.ts';

export const INVOICE_CURRENCY: string = 'INR';

export interface InvoiceProfileInfoResponse {
    readonly businessName: string;
    readonly addressLines: string;
    readonly phone: string;
    readonly website: string;
    readonly logoDataUrl: string;
    readonly defaultNotes: string;
    readonly numberPrefix: string;
    readonly defaultTermsDays: number;
}

export type InvoiceProfileUpdateRequest = InvoiceProfileInfoResponse;

export interface CustomerInfoResponse {
    readonly id: string;
    readonly name: string;
    readonly contactName: string;
    readonly addressLines: string;
    readonly email: string;
}

export interface CustomerCreateRequest {
    readonly name: string;
    readonly contactName: string;
    readonly addressLines: string;
    readonly email: string;
}

export interface CustomerModifyRequest extends CustomerCreateRequest {
    readonly id: string;
}

export interface InvoiceLine {
    itemName: string;
    description: string;
    quantity: number;
    rate: number;
    amount: number;
}

export interface InvoicePayment {
    readonly transactionId: string;
    readonly amount: number;
    readonly transactionTime: number;
    readonly accountId: string;
    readonly comment: string;
}

export interface InvoiceInfoResponse {
    readonly id: string;
    readonly kind: string;
    readonly number: string;
    readonly customerId: string;
    readonly billToName: string;
    readonly billToContactName: string;
    readonly billToAddressLines: string;
    readonly billToEmail: string;
    readonly currency: string;
    readonly invoiceDate: TextualYearMonthDay;
    readonly dueDate: TextualYearMonthDay;
    readonly notes: string;
    readonly total: number;
    readonly received: number;
    readonly closedManually: boolean;
    readonly lines?: InvoiceLine[];
    readonly payments?: InvoicePayment[];
}

export interface InvoiceLineRequest {
    readonly itemName: string;
    readonly description: string;
    readonly quantity: number;
    readonly rate: number;
}

export interface InvoiceCreateRequest {
    readonly number: string;
    readonly customerId: string;
    readonly invoiceDate: string;
    readonly dueDate: string;
    readonly notes: string;
    readonly lines: InvoiceLineRequest[];
}

export interface InvoiceModifyRequest extends InvoiceCreateRequest {
    readonly id: string;
}

export class InvoiceProfile implements InvoiceProfileInfoResponse {
    public businessName: string = '';
    public addressLines: string = '';
    public phone: string = '';
    public website: string = '';
    public logoDataUrl: string = '';
    public defaultNotes: string = '';
    public numberPrefix: string = 'INV';
    public defaultTermsDays: number = 7;

    public fillFrom(other: InvoiceProfileInfoResponse): void {
        this.businessName = other.businessName;
        this.addressLines = other.addressLines;
        this.phone = other.phone;
        this.website = other.website;
        this.logoDataUrl = other.logoDataUrl;
        this.defaultNotes = other.defaultNotes;
        this.numberPrefix = other.numberPrefix;
        this.defaultTermsDays = other.defaultTermsDays;
    }

    public toUpdateRequest(): InvoiceProfileUpdateRequest {
        return {
            businessName: this.businessName,
            addressLines: this.addressLines,
            phone: this.phone,
            website: this.website,
            logoDataUrl: this.logoDataUrl,
            defaultNotes: this.defaultNotes,
            numberPrefix: this.numberPrefix.trim(),
            defaultTermsDays: this.defaultTermsDays
        };
    }

    public clone(): InvoiceProfile {
        return InvoiceProfile.of(this);
    }

    public static of(response: InvoiceProfileInfoResponse): InvoiceProfile {
        const profile = new InvoiceProfile();
        profile.fillFrom(response);
        return profile;
    }

    public static createNewProfile(): InvoiceProfile {
        return new InvoiceProfile();
    }
}

export class Customer implements CustomerInfoResponse {
    public id: string = '';
    public name: string = '';
    public contactName: string = '';
    public addressLines: string = '';
    public email: string = '';

    public fillFrom(other: CustomerInfoResponse): void {
        this.id = other.id;
        this.name = other.name;
        this.contactName = other.contactName;
        this.addressLines = other.addressLines;
        this.email = other.email;
    }

    public toCreateRequest(): CustomerCreateRequest {
        return {
            name: this.name.trim(),
            contactName: this.contactName,
            addressLines: this.addressLines,
            email: this.email
        };
    }

    public toModifyRequest(): CustomerModifyRequest {
        return {
            id: this.id,
            ...this.toCreateRequest()
        };
    }

    public clone(): Customer {
        return Customer.of(this);
    }

    public static of(response: CustomerInfoResponse): Customer {
        const customer = new Customer();
        customer.fillFrom(response);
        return customer;
    }

    public static ofMulti(responses: CustomerInfoResponse[]): Customer[] {
        return responses.map(response => Customer.of(response));
    }

    public static createNewCustomer(): Customer {
        return new Customer();
    }
}

export class Invoice implements InvoiceInfoResponse {
    public id: string = '';
    public kind: string = 'invoice';
    public number: string = '';
    public customerId: string = '';
    public billToName: string = '';
    public billToContactName: string = '';
    public billToAddressLines: string = '';
    public billToEmail: string = '';
    public currency: string = INVOICE_CURRENCY;
    public invoiceDate: TextualYearMonthDay = '0-0-0';
    public dueDate: TextualYearMonthDay = '0-0-0';
    public notes: string = '';
    public total: number = 0;
    public received: number = 0;
    public closedManually: boolean = false;
    public lines: InvoiceLine[] = [];
    public payments: InvoicePayment[] = [];

    public fillFrom(other: InvoiceInfoResponse): void {
        this.id = other.id;
        this.kind = other.kind;
        this.number = other.number;
        this.customerId = other.customerId;
        this.billToName = other.billToName;
        this.billToContactName = other.billToContactName;
        this.billToAddressLines = other.billToAddressLines;
        this.billToEmail = other.billToEmail;
        this.currency = other.currency;
        this.invoiceDate = other.invoiceDate;
        this.dueDate = other.dueDate;
        this.notes = other.notes;
        this.total = other.total;
        this.received = other.received;
        this.closedManually = other.closedManually;
        this.lines = (other.lines ?? []).map(line => ({ ...line }));
        this.payments = (other.payments ?? []).map(payment => ({ ...payment }));
    }

    public toCreateRequest(): InvoiceCreateRequest {
        return {
            number: this.number.trim(),
            customerId: this.customerId,
            invoiceDate: this.invoiceDate,
            dueDate: this.dueDate,
            notes: this.notes,
            lines: this.lines.map(line => ({
                itemName: line.itemName.trim(),
                description: line.description,
                quantity: line.quantity,
                rate: line.rate
            }))
        };
    }

    public toModifyRequest(): InvoiceModifyRequest {
        return {
            id: this.id,
            ...this.toCreateRequest()
        };
    }

    public clone(): Invoice {
        return Invoice.of(this);
    }

    // a new unsaved invoice with this invoice's customer, lines and notes (the "Duplicate" action)
    public duplicate(): Invoice {
        const invoice = Invoice.createNewInvoice();
        invoice.customerId = this.customerId;
        invoice.notes = this.notes;
        invoice.lines = this.lines.map(line => ({ ...line }));
        return invoice;
    }

    public static of(response: InvoiceInfoResponse): Invoice {
        const invoice = new Invoice();
        invoice.fillFrom(response);
        return invoice;
    }

    public static ofMulti(responses: InvoiceInfoResponse[]): Invoice[] {
        return responses.map(response => Invoice.of(response));
    }

    public static createNewInvoice(): Invoice {
        const invoice = new Invoice();
        invoice.lines = [Invoice.createNewLine()];
        return invoice;
    }

    public static createNewLine(): InvoiceLine {
        return { itemName: '', description: '', quantity: 1, rate: 0, amount: 0 };
    }
}
