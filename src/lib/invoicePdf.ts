// A failed request must never save an HTML or JSON error page with a .pdf extension.
export async function readInvoicePdf(data: Blob): Promise<Blob> {
    if (!(data instanceof Blob) || data.type.split(';')[0]?.trim().toLowerCase() !== 'application/pdf'
        || await data.slice(0, 5).text() !== '%PDF-') {
        throw { message: 'Unable to download invoice PDF' };
    }

    return data;
}
