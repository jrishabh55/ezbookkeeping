import { describe, expect, it } from 'vitest';

import { Product } from '@/models/product.ts';
import { Invoice } from '@/models/invoice.ts';
import { filterProductSuggestions, applyProductToLine } from '@/lib/product.ts';

function product(id: string, name: string, archived = false): Product {
    const result = Product.createNewProduct();
    result.id = id;
    result.name = name;
    result.archived = archived;
    return result;
}

describe('filterProductSuggestions', () => {
    const products = [product('1', 'Website Hosting'), product('2', 'Domain'), product('3', 'Old Domain', true), product('4', 'AMC Domain Support')];

    it('hides archived products and matches anywhere in the name, case-insensitively', () => {
        expect(filterProductSuggestions(products, 'domain').map(p => p.id)).toEqual(['2', '4']);
    });

    it('lists names starting with the query first', () => {
        expect(filterProductSuggestions(products, 'do').map(p => p.id)).toEqual(['2', '4']);
        expect(filterProductSuggestions(products, 'host').map(p => p.id)).toEqual(['1']);
    });

    it('lists every active product for an empty query, up to the limit', () => {
        expect(filterProductSuggestions(products, '  ').map(p => p.id)).toEqual(['4', '2', '1']);
        expect(filterProductSuggestions(products, '', 2).length).toBe(2);
    });
});

describe('applyProductToLine', () => {
    it('copies the product into the line and keeps the quantity', () => {
        const line = Invoice.createNewLine('998314');
        line.quantity = 300;
        const domain = product('2', 'Domain');
        domain.description = 'codeation.io, 1 year';
        domain.defaultRate = 150000;

        applyProductToLine(line, domain);

        expect(line).toEqual({ itemName: 'Domain', description: 'codeation.io, 1 year', quantity: 300, rate: 150000, amount: 0, sac: '998314', productId: '2' });
    });

    it('uses the product SAC when it has one', () => {
        const line = Invoice.createNewLine('998314');
        const hosting = product('1', 'Website Hosting');
        hosting.sac = '998315';

        applyProductToLine(line, hosting);

        expect(line.sac).toBe('998315');
    });
});
