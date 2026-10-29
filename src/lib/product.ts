import type { InvoiceLine } from '@/models/invoice.ts';
import type { Product } from '@/models/product.ts';

// active products whose name contains the query (case-insensitive), names starting with it first, then alphabetical
export function filterProductSuggestions(products: readonly Product[], query: string, limit: number = 8): Product[] {
    const needle = query.trim().toLowerCase();

    return products
        .filter(product => !product.archived && (!needle || product.name.toLowerCase().includes(needle)))
        .sort((a, b) => {
            const aStarts = needle && a.name.toLowerCase().startsWith(needle) ? 0 : 1;
            const bStarts = needle && b.name.toLowerCase().startsWith(needle) ? 0 : 1;
            return aStarts - bStarts || a.name.localeCompare(b.name);
        })
        .slice(0, limit);
}

// fills a line from a product; the quantity stays, the SAC only changes when the product has one, every field stays editable
export function applyProductToLine(line: InvoiceLine, product: Product): void {
    line.productId = product.id;
    line.itemName = product.name;
    line.description = product.description;
    line.rate = product.defaultRate;

    if (product.sac) {
        line.sac = product.sac;
    }
}
