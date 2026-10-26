import type { TextualYearMonthDay } from '@/core/datetime.ts';

export const INVOICE_CURRENCY: string = 'INR';

// mirrors models.INVOICE_TAX_TYPE_* in pkg/models/invoice.go
export type InvoiceTaxType = 'none' | 'igst' | 'cgst_sgst';

// the tax type choices with their locale keys, in menu order
export const INVOICE_TAX_TYPES: { readonly type: InvoiceTaxType, readonly name: string }[] = [
    { type: 'none', name: 'None' },
    { type: 'igst', name: 'IGST' },
    { type: 'cgst_sgst', name: 'CGST + SGST' }
];

export interface InvoiceProfileInfoResponse {
    readonly businessName: string;
    readonly addressLines: string;
    readonly phone: string;
    readonly website: string;
    readonly logoDataUrl: string;
    readonly defaultNotes: string;
    readonly numberPrefix: string;
    readonly defaultTermsDays: number;
    readonly gstin: string;
    readonly defaultSac: string;
}

export type InvoiceProfileUpdateRequest = InvoiceProfileInfoResponse;

export interface CustomerInfoResponse {
    readonly id: string;
    readonly name: string;
    readonly contactName: string;
    readonly addressLines: string;
    readonly email: string;
    readonly phone: string;
    readonly website: string;
    readonly accountNumber: string;
    readonly pan: string;
    readonly termsDays: number;
    readonly notes: string;
    readonly gstin: string;
    readonly taxType: InvoiceTaxType;
    readonly taxRate: number; // basis points
}

export interface CustomerCreateRequest {
    readonly name: string;
    readonly contactName: string;
    readonly addressLines: string;
    readonly email: string;
    readonly phone: string;
    readonly website: string;
    readonly accountNumber: string;
    readonly pan: string;
    readonly termsDays: number;
    readonly notes: string;
    readonly gstin: string;
    readonly taxType: InvoiceTaxType;
    readonly taxRate: number; // basis points
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
    sac: string;
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
    readonly billToPhone: string;
    readonly billToPan: string;
    readonly billToGstin: string;
    readonly currency: string;
    readonly invoiceDate: TextualYearMonthDay;
    readonly dueDate: TextualYearMonthDay;
    readonly notes: string;
    readonly total: number;
    readonly taxType: InvoiceTaxType;
    readonly taxRate: number; // basis points
    readonly subtotal: number; // 0 on invoices saved before GST
    readonly taxAmount: number;
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
    readonly sac: string;
}

export interface InvoiceCreateRequest {
    readonly number: string;
    readonly customerId: string;
    readonly invoiceDate: string;
    readonly dueDate: string;
    readonly notes: string;
    readonly taxType: InvoiceTaxType;
    readonly taxRate: number;
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
    public gstin: string = '';
    public defaultSac: string = '';

    public fillFrom(other: InvoiceProfileInfoResponse): void {
        this.businessName = other.businessName;
        this.addressLines = other.addressLines;
        this.phone = other.phone;
        this.website = other.website;
        this.logoDataUrl = other.logoDataUrl;
        this.defaultNotes = other.defaultNotes;
        this.numberPrefix = other.numberPrefix;
        this.defaultTermsDays = other.defaultTermsDays;
        this.gstin = other.gstin;
        this.defaultSac = other.defaultSac;
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
            defaultTermsDays: this.defaultTermsDays,
            gstin: this.gstin.trim().toUpperCase(),
            defaultSac: this.defaultSac.trim()
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
    public phone: string = '';
    public website: string = '';
    public accountNumber: string = '';
    public pan: string = '';
    public termsDays: number = 0; // 0 = the profile's default terms
    public notes: string = '';
    public gstin: string = '';
    public taxType: InvoiceTaxType = 'none';
    public taxRate: number = 0; // basis points

    public fillFrom(other: CustomerInfoResponse): void {
        this.id = other.id;
        this.name = other.name;
        this.contactName = other.contactName;
        this.addressLines = other.addressLines;
        this.email = other.email;
        this.phone = other.phone;
        this.website = other.website;
        this.accountNumber = other.accountNumber;
        this.pan = other.pan;
        this.termsDays = other.termsDays;
        this.notes = other.notes;
        this.gstin = other.gstin;
        this.taxType = other.taxType || 'none';
        this.taxRate = other.taxRate;
    }

    public toCreateRequest(): CustomerCreateRequest {
        return {
            name: this.name.trim(),
            contactName: this.contactName,
            addressLines: this.addressLines,
            email: this.email,
            phone: this.phone,
            website: this.website,
            accountNumber: this.accountNumber,
            pan: this.pan.trim().toUpperCase(),
            termsDays: this.termsDays,
            notes: this.notes,
            gstin: this.gstin.trim().toUpperCase(),
            taxType: this.taxType,
            taxRate: this.taxType === 'none' ? 0 : this.taxRate
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
    public billToPhone: string = '';
    public billToPan: string = '';
    public billToGstin: string = '';
    public currency: string = INVOICE_CURRENCY;
    public invoiceDate: TextualYearMonthDay = '0-0-0';
    public dueDate: TextualYearMonthDay = '0-0-0';
    public notes: string = '';
    public total: number = 0;
    public taxType: InvoiceTaxType = 'none';
    public taxRate: number = 0; // basis points
    public subtotal: number = 0;
    public taxAmount: number = 0;
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
        this.billToPhone = other.billToPhone;
        this.billToPan = other.billToPan;
        this.billToGstin = other.billToGstin;
        this.currency = other.currency;
        this.invoiceDate = other.invoiceDate;
        this.dueDate = other.dueDate;
        this.notes = other.notes;
        this.total = other.total;
        this.taxType = other.taxType || 'none'; // invoices saved before GST have no tax type
        this.taxRate = other.taxRate;
        this.subtotal = other.subtotal;
        this.taxAmount = other.taxAmount;
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
            taxType: this.taxType,
            taxRate: this.taxType === 'none' ? 0 : this.taxRate,
            lines: this.lines.map(line => ({
                itemName: line.itemName.trim(),
                description: line.description,
                quantity: line.quantity,
                rate: line.rate,
                sac: line.sac.trim()
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

    public static createNewLine(sac: string = ''): InvoiceLine {
        return { itemName: '', description: '', quantity: 100, rate: 0, amount: 0, sac }; // quantity in hundredths
    }
}

export interface RecurringInvoiceInfoResponse {
    readonly id: string;
    readonly sourceInvoiceId: string;
    readonly sourceInvoiceNumber: string;
    readonly customerName: string;
    readonly dayOfMonth: number;
    readonly nextDate: TextualYearMonthDay;
    readonly enabled: boolean;
    readonly lastInvoiceId: string;
    readonly lastRunUnixTime: number;
}

export interface RecurringInvoiceCreateRequest {
    readonly sourceInvoiceId: string;
    readonly dayOfMonth: number;
    readonly nextDate: string;
}

export interface RecurringInvoiceModifyRequest {
    readonly id: string;
    readonly dayOfMonth: number;
    readonly nextDate: string;
    readonly enabled: boolean;
}

export class RecurringInvoice implements RecurringInvoiceInfoResponse {
    public id: string = '';
    public sourceInvoiceId: string = '';
    public sourceInvoiceNumber: string = '';
    public customerName: string = '';
    public dayOfMonth: number = 1;
    public nextDate: TextualYearMonthDay = '0-0-0';
    public enabled: boolean = true;
    public lastInvoiceId: string = '';
    public lastRunUnixTime: number = 0;

    public fillFrom(other: RecurringInvoiceInfoResponse): void {
        this.id = other.id;
        this.sourceInvoiceId = other.sourceInvoiceId;
        this.sourceInvoiceNumber = other.sourceInvoiceNumber;
        this.customerName = other.customerName;
        this.dayOfMonth = other.dayOfMonth;
        this.nextDate = other.nextDate;
        this.enabled = other.enabled;
        this.lastInvoiceId = other.lastInvoiceId;
        this.lastRunUnixTime = other.lastRunUnixTime;
    }

    public toCreateRequest(): RecurringInvoiceCreateRequest {
        return {
            sourceInvoiceId: this.sourceInvoiceId,
            dayOfMonth: this.dayOfMonth,
            nextDate: this.nextDate
        };
    }

    public toModifyRequest(): RecurringInvoiceModifyRequest {
        return {
            id: this.id,
            dayOfMonth: this.dayOfMonth,
            nextDate: this.nextDate,
            enabled: this.enabled
        };
    }

    public clone(): RecurringInvoice {
        return RecurringInvoice.of(this);
    }

    public static of(response: RecurringInvoiceInfoResponse): RecurringInvoice {
        const recurring = new RecurringInvoice();
        recurring.fillFrom(response);
        return recurring;
    }

    public static ofMulti(responses: RecurringInvoiceInfoResponse[]): RecurringInvoice[] {
        return responses.map(response => RecurringInvoice.of(response));
    }

    // a new unsaved recurring invoice repeating the given invoice
    public static createNewRecurringInvoice(sourceInvoiceId: string, dayOfMonth: number, nextDate: TextualYearMonthDay): RecurringInvoice {
        const recurring = new RecurringInvoice();
        recurring.sourceInvoiceId = sourceInvoiceId;
        recurring.dayOfMonth = dayOfMonth;
        recurring.nextDate = nextDate;
        return recurring;
    }
}
