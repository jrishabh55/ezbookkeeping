import { ref } from 'vue';
import { defineStore } from 'pinia';

import {
    type InvoicePayment,
    type InvoiceEmail,
    type InvoiceSendRequest,
    type InvoiceEmailPreviewRequest,
    type InvoiceEmailPreviewResponse,
    type InvoiceEmailTemplateUpdateRequest,
    type InvoiceRenewal,
    type CustomerOverview,
    type InvoiceKind,
    InvoiceProfile,
    Customer,
    Invoice,
    RecurringInvoice
} from '@/models/invoice.ts';
import { type InvoiceLineGroup, Product } from '@/models/product.ts';
import type { EstimateStatus } from '@/lib/estimate.ts';

import { isEquals } from '@/lib/common.ts';
import services, { type ApiResponsePromise } from '@/lib/services.ts';
import logger from '@/lib/logger.ts';
import { readInvoicePdf } from '@/lib/invoicePdf.ts';

export const useInvoicesStore = defineStore('invoices', () => {
    const allInvoices = ref<Invoice[]>([]);
    const allEstimates = ref<Invoice[]>([]);
    const allInvoicesMap = ref<Record<string, Invoice>>({});
    const allCustomers = ref<Customer[]>([]);
    const allCustomersMap = ref<Record<string, Customer>>({});
    const customerListStateInvalid = ref<boolean>(true);
    const invoiceProfile = ref<InvoiceProfile | null>(null);
    const allRecurringInvoices = ref<RecurringInvoice[]>([]);
    const allProducts = ref<Product[]>([]);
    const allProductsMap = ref<Record<string, Product>>({});

    // unwraps the API envelope and normalises errors exactly like the other stores' inline handlers
    function request<T>(apiRequest: ApiResponsePromise<T>, failedMessage: string): Promise<T> {
        return new Promise((resolve, reject) => {
            apiRequest.then(response => {
                const data = response.data;

                if (!data || !data.success || data.result === undefined || data.result === null) {
                    reject({ message: failedMessage });
                    return;
                }

                resolve(data.result);
            }).catch(error => {
                logger.error(failedMessage, error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: failedMessage });
                } else {
                    reject(error);
                }
            });
        });
    }

    function loadInvoiceList(invoices: Invoice[]): void {
        allInvoices.value = invoices;
        allInvoicesMap.value = {};

        for (const invoice of invoices) {
            allInvoicesMap.value[invoice.id] = invoice;
        }
    }

    function loadCustomerList(customers: Customer[]): void {
        allCustomers.value = customers;
        allCustomersMap.value = {};

        for (const customer of customers) {
            allCustomersMap.value[customer.id] = customer;
        }
    }

    function removeInvoiceFromList(invoiceId: string): void {
        allInvoices.value = allInvoices.value.filter(invoice => invoice.id !== invoiceId);
        delete allInvoicesMap.value[invoiceId];
        allEstimates.value = allEstimates.value.filter(estimate => estimate.id !== invoiceId);
    }

    function removeCustomerFromList(customerId: string): void {
        allCustomers.value = allCustomers.value.filter(customer => customer.id !== customerId);
        delete allCustomersMap.value[customerId];
    }

    function resetInvoices(): void {
        allInvoices.value = [];
        allInvoicesMap.value = {};
        allEstimates.value = [];
        allCustomers.value = [];
        allCustomersMap.value = {};
        customerListStateInvalid.value = true;
        invoiceProfile.value = null;
        allRecurringInvoices.value = [];
        allProducts.value = [];
        allProductsMap.value = {};
    }

    // ponytail: the invoice list is never served from cache, because "received" depends on transactions edited on other pages
    function loadAllInvoices({ force }: { force?: boolean }): Promise<Invoice[]> {
        return request(services.getAllInvoices(), 'Unable to retrieve invoice list').then(result => {
            const invoices = Invoice.ofMulti(result);

            if (force && isEquals(allInvoices.value, invoices)) {
                return Promise.reject({ message: 'Invoice list is up to date', isUpToDate: true });
            }

            loadInvoiceList(invoices);
            return invoices;
        });
    }

    function getInvoice({ invoiceId }: { invoiceId: string }): Promise<Invoice> {
        return request(services.getInvoice({ id: invoiceId }), 'Unable to retrieve invoice').then(result => Invoice.of(result));
    }

    function getInvoiceEmail({ emailId }: { emailId: string }): Promise<InvoiceEmail> {
        return request(services.getInvoiceEmail({ id: emailId }), 'Unable to retrieve invoice email');
    }

    function saveInvoice({ invoice, isEdit }: { invoice: Invoice, isEdit: boolean }): Promise<Invoice> {
        const apiRequest = isEdit ? services.modifyInvoice(invoice.toModifyRequest()) : services.addInvoice(invoice.toCreateRequest());

        return request(apiRequest, isEdit ? 'Unable to save invoice' : 'Unable to add invoice').then(result => Invoice.of(result));
    }

    function deleteInvoice({ invoice, beforeResolve }: { invoice: Invoice, beforeResolve?: (done: () => void) => void }): Promise<boolean> {
        return request(services.deleteInvoice({ id: invoice.id }), 'Unable to delete this invoice').then(result => {
            if (beforeResolve) {
                beforeResolve(() => removeInvoiceFromList(invoice.id));
            } else {
                removeInvoiceFromList(invoice.id);
            }

            return result;
        });
    }

    function setInvoiceClosed({ invoice, closed }: { invoice: Invoice, closed: boolean }): Promise<boolean> {
        const apiRequest = closed ? services.closeInvoice({ id: invoice.id }) : services.reopenInvoice({ id: invoice.id });
        return request(apiRequest, 'Unable to save invoice');
    }

    function getNextInvoiceNumber({ invoiceDate, kind }: { invoiceDate: string, kind?: InvoiceKind }): Promise<string> {
        return request(services.getNextInvoiceNumber({ invoiceDate, kind }), 'Unable to get next invoice number');
    }

    function getInvoicePaymentSuggestions({ invoiceId }: { invoiceId: string }): Promise<InvoicePayment[]> {
        return request(services.getInvoicePaymentSuggestions({ id: invoiceId }), 'Unable to retrieve matching transactions');
    }

    // settledAmount (in invoice currency) only when the transaction's account is in another currency
    function linkInvoicePayment({ invoiceId, transactionId, settledAmount }: { invoiceId: string, transactionId: string, settledAmount?: number }): Promise<boolean> {
        return request(services.linkInvoicePayment({ invoiceId, transactionId, settledAmount }), 'Unable to link this transaction');
    }

    function unlinkInvoicePayment({ invoiceId, transactionId }: { invoiceId: string, transactionId: string }): Promise<boolean> {
        return request(services.unlinkInvoicePayment({ invoiceId, transactionId }), 'Unable to unlink this transaction');
    }

    // enabled=true issues a fresh token every time (first share or "Reset Link"); false disables the link; resolves the stored
    // token and, for an estimate, its status afterwards (enabling the link also sends a draft one)
    function updateInvoicePublicLink({ invoiceId, enabled }: { invoiceId: string, enabled: boolean }): Promise<{ token: string, estimateStatus: EstimateStatus | '' }> {
        return request(services.updateInvoicePublicLink({ id: invoiceId, enabled }), 'Unable to update public link')
            .then(result => ({ token: result.publicToken, estimateStatus: result.estimateStatus ?? '' }));
    }

    // emails the invoice (public link + PDF); resolves the invoice as stored afterwards (public token, sent time)
    function sendInvoice(req: InvoiceSendRequest): Promise<Invoice> {
        return request(services.sendInvoice(req), 'Unable to send invoice').then(result => Invoice.of(result));
    }

    function previewInvoiceEmail(req: InvoiceEmailPreviewRequest): Promise<InvoiceEmailPreviewResponse> {
        return request(services.previewInvoiceEmail(req), 'Unable to preview invoice email');
    }

    function loadAllCustomers({ force }: { force?: boolean }): Promise<Customer[]> {
        if (!force && !customerListStateInvalid.value) {
            return Promise.resolve(allCustomers.value);
        }

        return request(services.getAllCustomers(), 'Unable to retrieve customer list').then(result => {
            const customers = Customer.ofMulti(result);
            customerListStateInvalid.value = false;

            if (force && isEquals(allCustomers.value, customers)) {
                return Promise.reject({ message: 'Customer list is up to date', isUpToDate: true });
            }

            loadCustomerList(customers);
            return customers;
        });
    }

    function saveCustomer({ customer, isEdit }: { customer: Customer, isEdit: boolean }): Promise<Customer> {
        const apiRequest = isEdit ? services.modifyCustomer(customer.toModifyRequest()) : services.addCustomer(customer.toCreateRequest());

        return request(apiRequest, isEdit ? 'Unable to save customer' : 'Unable to add customer').then(result => {
            const saved = Customer.of(result);

            if (isEdit) {
                allCustomers.value = allCustomers.value.map(item => item.id === saved.id ? saved : item);
            } else {
                allCustomers.value = [...allCustomers.value, saved].sort((a, b) => a.name.localeCompare(b.name));
            }

            allCustomersMap.value[saved.id] = saved;
            return saved;
        });
    }

    function deleteCustomer({ customer, beforeResolve }: { customer: Customer, beforeResolve?: (done: () => void) => void }): Promise<boolean> {
        return request(services.deleteCustomer({ id: customer.id }), 'Unable to delete this customer').then(result => {
            if (beforeResolve) {
                beforeResolve(() => removeCustomerFromList(customer.id));
            } else {
                removeCustomerFromList(customer.id);
            }

            return result;
        });
    }

    function loadInvoiceProfile({ force }: { force?: boolean }): Promise<InvoiceProfile> {
        if (!force && invoiceProfile.value) {
            return Promise.resolve(invoiceProfile.value);
        }

        return request(services.getInvoiceProfile(), 'Unable to retrieve invoice profile').then(result => {
            invoiceProfile.value = InvoiceProfile.of(result);
            return invoiceProfile.value;
        });
    }

    function updateInvoiceProfile({ profile }: { profile: InvoiceProfile }): Promise<InvoiceProfile> {
        return request(services.updateInvoiceProfile(profile.toBusinessProfileUpdateRequest()), 'Unable to save invoice profile').then(result => {
            invoiceProfile.value = InvoiceProfile.of(result);
            return invoiceProfile.value;
        });
    }

    function updateInvoiceEmailTemplate(req: InvoiceEmailTemplateUpdateRequest): Promise<InvoiceProfile> {
        return request(services.updateInvoiceEmailTemplate(req), 'Unable to save invoice email template').then(result => {
            invoiceProfile.value = InvoiceProfile.of(result);
            return invoiceProfile.value;
        });
    }

    // the recurring list is never served from cache: the cron job advances next dates behind the UI's back
    function loadAllRecurringInvoices({ force }: { force?: boolean }): Promise<RecurringInvoice[]> {
        return request(services.getAllRecurringInvoices(), 'Unable to retrieve recurring invoice list').then(result => {
            const recurrings = RecurringInvoice.ofMulti(result);

            if (force && isEquals(allRecurringInvoices.value, recurrings)) {
                return Promise.reject({ message: 'Recurring invoice list is up to date', isUpToDate: true });
            }

            allRecurringInvoices.value = recurrings;
            return recurrings;
        });
    }

    function addRecurringInvoice({ recurring }: { recurring: RecurringInvoice }): Promise<RecurringInvoice> {
        return request(services.addRecurringInvoice(recurring.toCreateRequest()), 'Unable to add recurring invoice').then(result => {
            const saved = RecurringInvoice.of(result);
            allRecurringInvoices.value = [...allRecurringInvoices.value, saved].sort((a, b) => a.nextDate.localeCompare(b.nextDate));
            return saved;
        });
    }

    function modifyRecurringInvoice({ recurring }: { recurring: RecurringInvoice }): Promise<RecurringInvoice> {
        return request(services.modifyRecurringInvoice(recurring.toModifyRequest()), 'Unable to save recurring invoice').then(result => {
            const saved = RecurringInvoice.of(result);
            allRecurringInvoices.value = allRecurringInvoices.value.map(item => item.id === saved.id ? saved : item);
            return saved;
        });
    }

    function deleteRecurringInvoice({ recurring, beforeResolve }: { recurring: RecurringInvoice, beforeResolve?: (done: () => void) => void }): Promise<boolean> {
        return request(services.deleteRecurringInvoice({ id: recurring.id }), 'Unable to delete this recurring invoice').then(result => {
            const remove = () => {
                allRecurringInvoices.value = allRecurringInvoices.value.filter(item => item.id !== recurring.id);
            };

            if (beforeResolve) {
                beforeResolve(remove);
            } else {
                remove();
            }

            return result;
        });
    }

    function loadProductList(products: Product[]): void {
        allProducts.value = products;
        allProductsMap.value = {};

        for (const product of products) {
            allProductsMap.value[product.id] = product;
        }
    }

    // ponytail: never cached, the list is small and also changes from other devices
    function loadAllProducts({ force }: { force?: boolean }): Promise<Product[]> {
        return request(services.getAllProducts(), 'Unable to retrieve product list').then(result => {
            const products = Product.ofMulti(result);

            if (force && isEquals(allProducts.value, products)) {
                return Promise.reject({ message: 'Product list is up to date', isUpToDate: true });
            }

            loadProductList(products);
            return products;
        });
    }

    function saveProduct({ product, isEdit }: { product: Product, isEdit: boolean }): Promise<Product> {
        const apiRequest = isEdit ? services.modifyProduct(product.toModifyRequest()) : services.addProduct(product.toCreateRequest());

        return request(apiRequest, isEdit ? 'Unable to save product' : 'Unable to add product').then(result => {
            const saved = Product.of(result);
            const others = allProducts.value.filter(item => item.id !== saved.id);
            loadProductList([...others, saved].sort((a, b) => a.name.localeCompare(b.name)));
            return saved;
        });
    }

    function removeProductFromList(productId: string): void {
        loadProductList(allProducts.value.filter(item => item.id !== productId));
    }

    function deleteProduct({ product, beforeResolve }: { product: Product, beforeResolve?: (done: () => void) => void }): Promise<boolean> {
        return request(services.deleteProduct({ id: product.id }), 'Unable to delete this product').then(result => {
            if (beforeResolve) {
                beforeResolve(() => removeProductFromList(product.id));
            } else {
                removeProductFromList(product.id);
            }

            return result;
        });
    }

    function getUnlinkedInvoiceLineGroups(): Promise<InvoiceLineGroup[]> {
        return request(services.getUnlinkedInvoiceLineGroups(), 'Unable to retrieve invoice lines');
    }

    function linkInvoiceLinesToProduct({ itemName, productId }: { itemName: string, productId: string }): Promise<number> {
        return request(services.linkInvoiceLinesToProduct({ itemName, productId }), 'Unable to link invoice lines');
    }

    function loadInvoiceRenewals(): Promise<InvoiceRenewal[]> {
        return request(services.getInvoiceRenewals(), 'Unable to retrieve renewals')
            .then(result => result.map(item => ({ state: item.state, recurringId: item.recurringId, invoice: Invoice.of(item.invoice) })));
    }

    function getCustomerOverview({ customerId }: { customerId: string }): Promise<CustomerOverview> {
        return request(services.getCustomerOverview({ id: customerId }), 'Unable to retrieve customer');
    }

    // like the invoice list, never served from cache: client answers and conversions change estimates behind the UI's back
    function loadAllEstimates({ force }: { force?: boolean }): Promise<Invoice[]> {
        return request(services.getAllEstimates(), 'Unable to retrieve estimate list').then(result => {
            const estimates = Invoice.ofMulti(result);

            if (force && isEquals(allEstimates.value, estimates)) {
                return Promise.reject({ message: 'Estimate list is up to date', isUpToDate: true });
            }

            allEstimates.value = estimates;
            return estimates;
        });
    }

    function setEstimateResponse({ estimateId, status }: { estimateId: string, status: 'accepted' | 'declined' }): Promise<Invoice> {
        return request(services.setEstimateResponse({ id: estimateId, status }), 'Unable to save estimate').then(result => Invoice.of(result));
    }

    function reopenEstimate({ estimateId }: { estimateId: string }): Promise<Invoice> {
        return request(services.reopenEstimate({ id: estimateId }), 'Unable to save estimate').then(result => Invoice.of(result));
    }

    // the unsaved invoice an estimate (milestoneId '') or one of its milestones turns into, for the editor
    function getEstimateInvoiceDraft({ estimateId, milestoneId }: { estimateId: string, milestoneId: string }): Promise<Invoice> {
        return request(services.getEstimateInvoiceDraft({ id: estimateId, milestoneId }), 'Unable to convert estimate').then(result => Invoice.of(result));
    }

    function loadEstimatesToInvoice(): Promise<Invoice[]> {
        return request(services.getEstimatesToInvoice(), 'Unable to retrieve estimate list').then(result => Invoice.ofMulti(result));
    }

    async function getInvoicePdf(invoiceId: string): Promise<Blob> {
        try {
            const response = await services.getInvoicePdf(invoiceId);
            return await readInvoicePdf(response.data);
        } catch (error) {
            logger.error('Unable to download invoice PDF', error);
            throw { message: 'Unable to download invoice PDF' };
        }
    }

    return {
        // states
        allInvoices,
        allEstimates,
        allInvoicesMap,
        allCustomers,
        allCustomersMap,
        customerListStateInvalid,
        invoiceProfile,
        allRecurringInvoices,
        allProducts,
        allProductsMap,
        // functions
        resetInvoices,
        loadAllInvoices,
        getInvoice,
        saveInvoice,
        deleteInvoice,
        setInvoiceClosed,
        getNextInvoiceNumber,
        getInvoicePaymentSuggestions,
        linkInvoicePayment,
        unlinkInvoicePayment,
        updateInvoicePublicLink,
        sendInvoice,
        getInvoiceEmail,
        previewInvoiceEmail,
        loadAllCustomers,
        saveCustomer,
        deleteCustomer,
        getInvoicePdf,
        loadInvoiceProfile,
        updateInvoiceProfile,
        updateInvoiceEmailTemplate,
        loadAllRecurringInvoices,
        addRecurringInvoice,
        modifyRecurringInvoice,
        deleteRecurringInvoice,
        loadAllProducts,
        saveProduct,
        deleteProduct,
        getUnlinkedInvoiceLineGroups,
        linkInvoiceLinesToProduct,
        loadInvoiceRenewals,
        getCustomerOverview,
        loadAllEstimates,
        setEstimateResponse,
        reopenEstimate,
        getEstimateInvoiceDraft,
        loadEstimatesToInvoice
    };
});
