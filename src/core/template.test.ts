import { describe, expect, it } from 'vitest';
import { TemplateType, isScheduleLike } from './template.ts';

describe('isScheduleLike', () => {
    it('is true for schedule and forecast templates only', () => {
        expect(isScheduleLike(TemplateType.Schedule.type)).toBe(true);
        expect(isScheduleLike(TemplateType.Forecast.type)).toBe(true);
        expect(isScheduleLike(TemplateType.Normal.type)).toBe(false);
    });
});
