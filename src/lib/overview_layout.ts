import { type PartialRecord, entries } from '@/core/base.ts';
import { type StartEndTime, type WeekDayValue, DateRange } from '@/core/datetime.ts';
import {
    type OverviewWidgetSettingValue,
    type OverviewWidgetSettingItem,
    type OverviewWidgetDefinitionBase,
    type OverviewWidgetSettingContext,
    type OverviewRecentTransactionsQuery,
    type OverviewLayoutBase,
    type OverviewWidgetLayoutBase,
    type OverviewHomeFilter,
    type OverviewWidgetAmountsQuery,
    type OverviewWidgetStatisticsQuery,
    type OverviewNamedRange,
    OverviewComparisonMode,
    type DesktopOverviewLayout,
    type DesktopOverviewWidgetLayout,
    type MobileOverviewLayout,
    type MobileOverviewWidgetLayout,
    OverviewWidgetType,
    OverviewWidgetDataRequirement
} from '@/core/overview_layout.ts';

import {
    DESKTOP_OVERVIEW_LAYOUT_COLUMNS,
    DESKTOP_OVERVIEW_LAYOUT_MAX_WIDGETS,
    DESKTOP_OVERVIEW_LAYOUT_MAX_ROWS,
    DESKTOP_OVERVIEW_WIDGET_DEFINITIONS,
    DEFAULT_DESKTOP_OVERVIEW_LAYOUT,
    MOBILE_OVERVIEW_LAYOUT_MAX_WIDGETS,
    MOBILE_OVERVIEW_WIDGET_DEFINITIONS,
    DEFAULT_MOBILE_OVERVIEW_LAYOUT,
    RECENT_TRANSACTIONS_WIDGET_DEFAULT_ITEM_COUNT
} from '@/consts/overview_layout.ts';

import { AmountFilterType } from '@/core/numeral.ts';
import {
    TransactionTagFilter,
    LATEST_12MONTHS_TRANSACTION_AMOUNTS_REQUEST_TYPES,
    type TransactionAmountsRequestType
} from '@/models/transaction.ts';

import {
    isDefined,
    isObject,
    isArray,
    isString,
    isNumber,
    isInteger,
    isBoolean,
    isHextualColor,
    normalizeInteger,
    objectFieldToArrayItem
} from '@/lib/common.ts';

import {
    isTransactionFromAITextRecognitionEnabled,
    isTransactionFromAIImageRecognitionEnabled
} from '@/lib/server_settings.ts';

import { getDateRangeByDateType } from '@/lib/datetime.ts';

import {
    getWidgetFilter,
    isWidgetFilterEmpty,
    getExcludedIds,
    mergeTagFilters,
    getComparisonRange,
    getComparisonUnit
} from '@/lib/overview_widget_query.ts';

function normalizeOverviewWidgetSettings(definition: OverviewWidgetDefinitionBase, settings: unknown): Record<string, OverviewWidgetSettingValue> {
    const normalized = { ...definition.defaultSettings };
    const source: Record<string, unknown> = isObject(settings) ? settings as Record<string, unknown> : {};

    for (const setting of definition.supportsSettings) {
        const sourceValue = source[setting.settingName];
        const normalizedValue = normalizeOverviewWidgetSetting(setting, sourceValue);

        if (isDefined(normalizedValue) &&
            ((isString(normalizedValue) && normalizedValue.trim() !== '' && normalizedValue.trim() !== normalized[setting.settingName])
                || isNumber(normalizedValue)
                || isBoolean(normalizedValue)
                || isArray(normalizedValue)
            )) {
            normalized[setting.settingName] = normalizedValue;
        }
    }

    return normalized;
}

function normalizeOverviewWidgetSetting(setting: OverviewWidgetSettingItem, value: unknown): OverviewWidgetSettingValue | undefined {
    if (setting.settingType === 'itemCountSelect') {
        return isInteger(value) && setting.itemCountValues.includes(value) ? value : undefined;
    } else if (setting.settingType === 'monthSelect') {
        return isInteger(value) && setting.monthValues.includes(value) ? value : undefined;
    } else if (setting.settingType === 'accountSelect') {
        return isArray(value) ? value.filter(id => isString(id)) : undefined;
    } else if (setting.settingType === 'categorySelect') {
        return isArray(value) ? value.filter(id => isString(id)) : undefined;
    } else if (setting.settingType === 'tagSelect') {
        return isString(value) && TransactionTagFilter.parse(value) ? value : undefined;
    } else if (setting.settingType === 'customSelect') {
        if (!setting.multiple) {
            return (isString(value) || isNumber(value)) && setting.selectValues.some(item => item.value === value) ? value : undefined;
        } else if (setting.multiple) {
            if (!isArray(value)) {
                return undefined;
            }

            const allowedValues: Record<string, boolean> = {};

            for (const item of setting.selectValues) {
                allowedValues[item.value] = true;
            }

            const selectedValues: (string | number)[] = [];
            const selectedValuesMap: Record<string, boolean> = {};

            for (const item of value) {
                if (isDefined(setting.allValue) && item === setting.allValue) {
                    return [setting.allValue];
                }

                if ((isString(item) || isNumber(item)) && allowedValues[item] && !selectedValuesMap[item]) {
                    selectedValues.push(item);
                    selectedValuesMap[item] = true;
                }
            }

            return selectedValues.length >= (setting.minSelections ?? 1) ? selectedValues : undefined;
        } else {
            return undefined;
        }
    } else if (setting.settingType === 'switch') {
        return isBoolean(value) ? value : undefined;
    } else if (setting.settingType === 'color') {
        return isHextualColor(value) ? value.toLowerCase() : undefined;
    } else if (setting.settingType === 'amount') {
        return isInteger(value) ? value : undefined;
    } else if (setting.settingType === 'amountFilter') {
        return isString(value) && AmountFilterType.parseTextualFilter(value) ? value : undefined;
    } else if (setting.settingType === 'textbox') {
        return isString(value) ? value : undefined;
    } else {
        return undefined;
    }
}

function getMaximumWidgetMonths(layout: OverviewLayoutBase, type: OverviewWidgetType): number {
    let months: number = 6;

    for (const widget of layout.widgets) {
        if (widget.type === type) {
            const monthsValue = widget.settings['months'];

            if (isInteger(monthsValue)) {
                months = Math.max(months, monthsValue);
            }
        }
    }

    return months;
}

function isDesktopWidgetsOverlap(first: DesktopOverviewWidgetLayout, second: DesktopOverviewWidgetLayout): boolean {
    return first.x < second.x + second.w && first.x + first.w > second.x && first.y < second.y + second.h && first.y + first.h > second.y;
}

export function cloneWidget<T extends OverviewWidgetLayoutBase>(widget: T): T {
    const settings: Record<string, OverviewWidgetSettingValue> = {};

    for (const [key, value] of entries(widget.settings)) {
        settings[key] = isArray(value) ? [...value] : value;
    }

    return { ...widget, settings };
}

export function getOverviewDataRequirements(layout: OverviewLayoutBase, definitions: PartialRecord<OverviewWidgetType, OverviewWidgetDefinitionBase>): PartialRecord<OverviewWidgetDataRequirement, boolean> {
    const requirements: PartialRecord<OverviewWidgetDataRequirement, boolean> = {};

    for (const widget of layout.widgets) {
        const definition = definitions[widget.type];

        if (!definition) {
            continue;
        }

        for (const requirement of definition.dataRequirements) {
            requirements[requirement] = true;
        }
    }

    if (requirements[OverviewWidgetDataRequirement.TransactionOverviewLast12Months]) {
        requirements[OverviewWidgetDataRequirement.TransactionOverview] = true;
    }

    if (requirements[OverviewWidgetDataRequirement.TransactionOverviewLast2Months]) {
        requirements[OverviewWidgetDataRequirement.TransactionOverview] = true;
    }

    return requirements;
}

export function getOverviewWidgetSettingContext(): OverviewWidgetSettingContext {
    return {
        aiTextRecognitionEnabled: isTransactionFromAITextRecognitionEnabled(),
        aiImageRecognitionEnabled: isTransactionFromAIImageRecognitionEnabled()
    };
}

export function getOverviewTransactionOverviewMonths(layout: OverviewLayoutBase): number {
    let months: number = 1;

    for (const widget of layout.widgets) {
        if (widget.type === OverviewWidgetType.IncomeExpenseTrend) {
            const monthsValue = widget.settings['months'];

            if (isInteger(monthsValue)) {
                months = Math.max(months, monthsValue);
            }
        } else if (widget.type === OverviewWidgetType.CurrentMonthExpenseProgress) {
            months = Math.max(months, 2);
        }
    }

    return months;
}

export function getOverviewUnreconciledTransactionAccountIds(layout: OverviewLayoutBase): string[] {
    const accountIds: Record<string, boolean> = {};

    for (const widget of layout.widgets) {
        if (widget.type !== OverviewWidgetType.AccountUnreconciledTransactions) {
            continue;
        }

        const widgetAccountIds = widget.settings['accountIds'];

        if (!isArray(widgetAccountIds) || !widgetAccountIds.length) {
            return [];
        }

        for (const accountId of widgetAccountIds as string[]) {
            accountIds[accountId] = true;
        }
    }

    return objectFieldToArrayItem(accountIds);
}

export function getOverviewRecentTransactionsQuery(settings: Record<string, OverviewWidgetSettingValue>): OverviewRecentTransactionsQuery {
    return {
        count: isInteger(settings['itemCount']) ? settings['itemCount'] : RECENT_TRANSACTIONS_WIDGET_DEFAULT_ITEM_COUNT,
        accountIds: isArray(settings['accountIds']) ? settings['accountIds'] as string[] : [],
        categoryIds: isArray(settings['categoryIds']) ? settings['categoryIds'] as string[] : [],
        tagFilter: isString(settings['tagFilter']) ? settings['tagFilter'] : '',
        amountFilter: isString(settings['amountFilter']) ? settings['amountFilter'] : '',
        keyword: isString(settings['keyword']) ? settings['keyword'] : ''
    };
}

export function getOverviewRecentTransactionsQueries(layout: OverviewLayoutBase): Record<string, OverviewRecentTransactionsQuery> {
    const queries: Record<string, OverviewRecentTransactionsQuery> = {};

    for (const widget of layout.widgets) {
        if (widget.type === OverviewWidgetType.RecentTransactions) {
            queries[widget.id] = getOverviewRecentTransactionsQuery(widget.settings);
        }
    }

    return queries;
}

export function getOverviewAssetTrendMonths(layout: OverviewLayoutBase): number {
    return getMaximumWidgetMonths(layout, OverviewWidgetType.NetAssetsTrend);
}

export function getOverviewCalendarHeatmapMonths(layout: OverviewLayoutBase): number {
    return getMaximumWidgetMonths(layout, OverviewWidgetType.TransactionCalendarHeatmap);
}

export function getOverviewTransactionCategoryStatisticDateTypes(layout: OverviewLayoutBase): number[] {
    const dateTypes: number[] = [];
    const existingDateTypes: Record<number, boolean> = {};

    for (const widget of layout.widgets) {
        if (widget.type !== OverviewWidgetType.ExpenseCategoryRanking) {
            continue;
        }

        const dateType = widget.settings['dateRange'];

        // the shared loader only supports this month / this year; fiscal year ranges use the per-widget statistics query
        if (isNumber(dateType) && (dateType === DateRange.ThisMonth.type || dateType === DateRange.ThisYear.type) && !existingDateTypes[dateType]) {
            dateTypes.push(dateType);
            existingDateTypes[dateType] = true;
        }
    }

    return dateTypes;
}

export function resolveDesktopOverviewWidgetCollisions(widgets: DesktopOverviewWidgetLayout[], activeId?: string): DesktopOverviewWidgetLayout[] {
    const sorted: DesktopOverviewWidgetLayout[] = widgets.map(cloneWidget).sort((a, b) => {
        if (a.id === activeId) {
            return -1;
        } else if (b.id === activeId) {
            return 1;
        } else {
            return a.y - b.y || a.x - b.x || a.id.localeCompare(b.id);
        }
    });
    const placed: DesktopOverviewWidgetLayout[] = [];

    for (const widget of sorted) {
        let overlapping: DesktopOverviewWidgetLayout[] = placed.filter(other => isDesktopWidgetsOverlap(widget, other));

        while (overlapping.length > 0) {
            widget.y = Math.max(...overlapping.map(other => other.y + other.h));
            overlapping = placed.filter(other => isDesktopWidgetsOverlap(widget, other));
        }

        placed.push(widget);
    }

    return placed;
}

export function compactDesktopOverviewWidgets(widgets: DesktopOverviewWidgetLayout[], fixedId?: string): DesktopOverviewWidgetLayout[] {
    const result: DesktopOverviewWidgetLayout[] = widgets.map(cloneWidget).sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id));

    for (const widget of result) {
        if (widget.id === fixedId) {
            continue;
        }

        if (fixedId) {
            for (let y = 0; y < widget.y; y++) {
                const candidate = { ...widget, y };

                if (!result.some(other => other.id !== widget.id && isDesktopWidgetsOverlap(candidate, other))) {
                    widget.y = y;
                    break;
                }
            }

            continue;
        }

        while (widget.y > 0) {
            const candidate = { ...widget, y: widget.y - 1 };

            if (result.some(other => other.id !== widget.id && isDesktopWidgetsOverlap(candidate, other))) {
                break;
            }

            widget.y--;
        }
    }

    return result.sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id));
}

export function findDesktopOverviewWidgetPosition(widgets: DesktopOverviewWidgetLayout[], width: number, height: number): { x: number; y: number } {
    for (let y = 0; y < DESKTOP_OVERVIEW_LAYOUT_MAX_ROWS - height; y++) {
        for (let x = 0; x <= DESKTOP_OVERVIEW_LAYOUT_COLUMNS - width; x++) {
            const candidate: DesktopOverviewWidgetLayout = {
                id: '',
                type: OverviewWidgetType.CurrentMonthOverview,
                x,
                y,
                w: width,
                h: height,
                settings: {}
            };

            if (!widgets.some(widget => isDesktopWidgetsOverlap(candidate, widget))) {
                return { x, y };
            }
        }
    }

    return { x: 0, y: Math.max(0, ...widgets.map(widget => widget.y + widget.h)) };
}

export function findDesktopOverviewWidgetDuplicatePosition(widgets: DesktopOverviewWidgetLayout[], widget: DesktopOverviewWidgetLayout): { x: number; y: number } {
    const rightPosition = { x: widget.x + widget.w, y: widget.y };
    const rightCandidate = { ...widget, ...rightPosition };

    if (rightCandidate.x + rightCandidate.w <= DESKTOP_OVERVIEW_LAYOUT_COLUMNS &&
        !widgets.some(currentWidget => isDesktopWidgetsOverlap(rightCandidate, currentWidget))) {
        return rightPosition;
    }

    return { x: widget.x, y: widget.y + widget.h };
}

export function isDefaultDesktopOverviewLayout(layout: DesktopOverviewLayout): boolean {
    return serializeDesktopOverviewLayout(layout) === serializeDesktopOverviewLayout(DEFAULT_DESKTOP_OVERVIEW_LAYOUT);
}

export function normalizeDesktopOverviewLayout(input: unknown): DesktopOverviewLayout {
    if (!isObject(input)) {
        throw new Error('input is not an object');
    }

    const source = input as Record<string, unknown>;
    const sourceWidgets = source['widgets'];

    if (!isArray(sourceWidgets)) {
        throw new Error('widgets is not an array');
    }

    const finalWidgets: DesktopOverviewWidgetLayout[] = [];
    const existsIds: Record<string, boolean> = {};

    for (const item of sourceWidgets) {
        if (!isObject(item)) {
            continue;
        }

        if (finalWidgets.length >= DESKTOP_OVERVIEW_LAYOUT_MAX_WIDGETS) {
            break;
        }

        const sourceWidget = item as Record<string, unknown>;
        const type = sourceWidget['type'] as OverviewWidgetType;
        const definition = DESKTOP_OVERVIEW_WIDGET_DEFINITIONS[type];
        const widgetId = sourceWidget['id'];

        if (!definition || !widgetId || !isString(widgetId) || widgetId.length > 100 || existsIds[widgetId]) {
            continue;
        }

        const w: number = normalizeInteger(sourceWidget['w'], definition.defaultWidth, definition.minWidth, Math.min(DESKTOP_OVERVIEW_LAYOUT_COLUMNS, definition.maxWidth ?? DESKTOP_OVERVIEW_LAYOUT_COLUMNS));
        const h: number = normalizeInteger(sourceWidget['h'], definition.defaultHeight, definition.minHeight, Math.min(DESKTOP_OVERVIEW_LAYOUT_MAX_ROWS, definition.maxHeight ?? DESKTOP_OVERVIEW_LAYOUT_MAX_ROWS));
        const x: number = normalizeInteger(sourceWidget['x'], 0, 0, DESKTOP_OVERVIEW_LAYOUT_COLUMNS - w);
        const y: number = normalizeInteger(sourceWidget['y'], 0, 0, DESKTOP_OVERVIEW_LAYOUT_MAX_ROWS - h);

        const finalWidget: DesktopOverviewWidgetLayout = {
            id: widgetId,
            type: type,
            x: x,
            y: y,
            w: w,
            h: h,
            settings: normalizeOverviewWidgetSettings(definition, sourceWidget['settings'])
        };

        existsIds[widgetId] = true;
        finalWidgets.push(finalWidget);
    }

    return {
        widgets: compactDesktopOverviewWidgets(resolveDesktopOverviewWidgetCollisions(finalWidgets))
    };
}

export function cloneDesktopOverviewLayout(original: DesktopOverviewLayout): DesktopOverviewLayout {
    return {
        widgets: original.widgets.map(cloneWidget)
    };
}

export function parseDesktopOverviewLayout(value: string): DesktopOverviewLayout {
    if (!value) {
        return cloneDesktopOverviewLayout(DEFAULT_DESKTOP_OVERVIEW_LAYOUT);
    }

    return normalizeDesktopOverviewLayout(JSON.parse(value));
}

export function serializeDesktopOverviewLayout(layout: DesktopOverviewLayout, pretty?: boolean): string {
    const normalized = normalizeDesktopOverviewLayout(layout);
    return JSON.stringify(normalized, null, pretty ? 4 : undefined);
}

export function isDefaultMobileOverviewLayout(layout: MobileOverviewLayout): boolean {
    return serializeMobileOverviewLayout(layout) === serializeMobileOverviewLayout(DEFAULT_MOBILE_OVERVIEW_LAYOUT);
}

export function normalizeMobileOverviewLayout(input: unknown): MobileOverviewLayout {
    if (!isObject(input)) {
        throw new Error('input is not an object');
    }

    const source = input as Record<string, unknown>;
    const sourceWidgets = source['widgets'];

    if (!isArray(sourceWidgets)) {
        throw new Error('widgets is not an array');
    }

    const finalWidgets: MobileOverviewWidgetLayout[] = [];
    const existsIds: Record<string, boolean> = {};

    for (const item of sourceWidgets) {
        if (!isObject(item)) {
            continue;
        }

        if (finalWidgets.length >= MOBILE_OVERVIEW_LAYOUT_MAX_WIDGETS) {
            break;
        }

        const sourceWidget = item as Record<string, unknown>;
        const type = sourceWidget['type'] as OverviewWidgetType;
        const definition = MOBILE_OVERVIEW_WIDGET_DEFINITIONS[type];
        const widgetId = sourceWidget['id'];

        if (!definition || !widgetId || !isString(widgetId) || widgetId.length > 100 || existsIds[widgetId]) {
            continue;
        }

        const finalWidget: MobileOverviewWidgetLayout = {
            id: widgetId,
            type: type,
            settings: normalizeOverviewWidgetSettings(definition, sourceWidget['settings'])
        }

        existsIds[widgetId] = true;
        finalWidgets.push(finalWidget);
    }

    return {
        widgets: finalWidgets
    };
}

export function cloneMobileOverviewLayout(original: MobileOverviewLayout): MobileOverviewLayout {
    return {
        widgets: original.widgets.map(cloneWidget)
    };
}

export function parseMobileOverviewLayout(value: string): MobileOverviewLayout {
    if (!value) {
        return cloneMobileOverviewLayout(DEFAULT_MOBILE_OVERVIEW_LAYOUT);
    }

    return normalizeMobileOverviewLayout(JSON.parse(value));
}

export function serializeMobileOverviewLayout(layout: MobileOverviewLayout, pretty?: boolean): string {
    const normalized = normalizeMobileOverviewLayout(layout);
    return JSON.stringify(normalized, null, pretty ? 2 : undefined);
}

export const OVERVIEW_HOME_DASHBOARD_ID = 'home';
export const OVERVIEW_MAX_EXTRA_DASHBOARDS = 20;
const OVERVIEW_DASHBOARD_NAME_MAX_LENGTH = 40;

export interface OverviewDashboard<L extends OverviewLayoutBase> {
    id: string;
    name: string;
    layout: L;
}

export interface OverviewWidgetQueryContext {
    allAccountIds: string[];
    allCategoryIds: string[];
    homeFilter: OverviewHomeFilter | null;
    todayLastUnixTime: number;
    firstDayOfWeek: WeekDayValue;
    fiscalYearStart: number;
    monthRanges: Record<TransactionAmountsRequestType, StartEndTime>;
}

const SHARED_OVERVIEW_PERIOD_DATE_RANGES: number[] = [
    DateRange.Today.type,
    DateRange.ThisWeek.type,
    DateRange.ThisMonth.type,
    DateRange.ThisYear.type
];

function getWidgetExclusionsAndTags(settings: OverviewWidgetLayoutBase['settings'], ctx: OverviewWidgetQueryContext): { excludeAccountIds: string[]; excludeCategoryIds: string[]; tagFilter: string; filterEmpty: boolean } {
    const filter = getWidgetFilter(settings);
    const homeFilter = ctx.homeFilter;

    return {
        excludeAccountIds: [...new Set([...(homeFilter?.excludeAccountIds ?? []), ...getExcludedIds(filter.accountIds, ctx.allAccountIds)])],
        excludeCategoryIds: [...new Set([...(homeFilter?.excludeCategoryIds ?? []), ...getExcludedIds(filter.categoryIds, ctx.allCategoryIds)])],
        tagFilter: mergeTagFilters(homeFilter?.tagFilter ?? '', filter.tagFilter),
        filterEmpty: isWidgetFilterEmpty(filter)
    };
}

function getWidgetCurrentRange(settings: OverviewWidgetLayoutBase['settings'], ctx: OverviewWidgetQueryContext): { dateType: number; range: OverviewNamedRange } | null {
    const dateType = settings['dateRange'];

    if (!isInteger(dateType)) {
        return null;
    }

    const range = getDateRangeByDateType(dateType, ctx.firstDayOfWeek, ctx.fiscalYearStart);

    if (!range) {
        return null;
    }

    return { dateType, range: { name: 'current', startTime: range.minTime, endTime: range.maxTime } };
}

function getTrendRangeNames(settings: OverviewWidgetLayoutBase['settings']): TransactionAmountsRequestType[] {
    const monthsValue = settings['months'];
    const months = normalizeInteger(isInteger(monthsValue) ? monthsValue : 12, 12, 1, 12);

    return [...LATEST_12MONTHS_TRANSACTION_AMOUNTS_REQUEST_TYPES.slice(-months, -1), 'thisMonth'];
}

export function getOverviewWidgetAmountsQueries(layout: OverviewLayoutBase, ctx: OverviewWidgetQueryContext): Record<string, OverviewWidgetAmountsQuery> {
    const queries: Record<string, OverviewWidgetAmountsQuery> = {};

    for (const widget of layout.widgets) {
        const settings = widget.settings;
        const common = getWidgetExclusionsAndTags(settings, ctx);
        let ranges: OverviewNamedRange[] | null = null;

        if (widget.type === OverviewWidgetType.KeyFigure) {
            const current = getWidgetCurrentRange(settings, ctx);

            if (current) {
                ranges = [current.range];
                const comparison = settings['comparison'] as OverviewComparisonMode;
                const unit = getComparisonUnit(current.dateType);
                const compare = unit ? getComparisonRange(current.range, ctx.todayLastUnixTime, unit, comparison) : null;

                if (compare) {
                    ranges.push({ name: 'compare', startTime: compare.startTime, endTime: compare.endTime });
                }
            }
        } else if (widget.type === OverviewWidgetType.PeriodIncomeExpense || widget.type === OverviewWidgetType.PeriodNetIncomeAndSavingsRate) {
            const current = getWidgetCurrentRange(settings, ctx);

            if (current && (!common.filterEmpty || !SHARED_OVERVIEW_PERIOD_DATE_RANGES.includes(current.dateType) || !ctx.homeFilter)) {
                ranges = [current.range];
            }
        } else if (widget.type === OverviewWidgetType.IncomeExpenseTrend) {
            if (!common.filterEmpty || !ctx.homeFilter) {
                ranges = getTrendRangeNames(settings).map(name => ({ name, startTime: ctx.monthRanges[name].startTime, endTime: ctx.monthRanges[name].endTime }));
            }
        }

        if (ranges) {
            queries[widget.id] = {
                ranges,
                excludeAccountIds: common.excludeAccountIds,
                excludeCategoryIds: common.excludeCategoryIds,
                tagFilter: common.tagFilter
            };
        }
    }

    return queries;
}

export function getOverviewWidgetStatisticsQueries(layout: OverviewLayoutBase, ctx: OverviewWidgetQueryContext): Record<string, OverviewWidgetStatisticsQuery> {
    const queries: Record<string, OverviewWidgetStatisticsQuery> = {};

    for (const widget of layout.widgets) {
        if (widget.type !== OverviewWidgetType.ExpenseCategoryRanking) {
            continue;
        }

        const filter = getWidgetFilter(widget.settings);
        const current = getWidgetCurrentRange(widget.settings, ctx);

        if (current && (filter.tagFilter || current.dateType === DateRange.ThisFiscalYear.type || current.dateType === DateRange.LastFiscalYear.type || !ctx.homeFilter)) {
            queries[widget.id] = {
                startTime: current.range.startTime,
                endTime: current.range.endTime,
                tagFilter: mergeTagFilters(ctx.homeFilter?.tagFilter ?? '', filter.tagFilter)
            };
        }
    }

    return queries;
}

function parseOverviewExtraDashboards<L extends OverviewLayoutBase>(value: string, normalize: (input: unknown) => L): OverviewDashboard<L>[] {
    let source: unknown;

    try {
        source = JSON.parse(value);
    } catch {
        return [];
    }

    if (!isArray(source)) {
        return [];
    }

    const dashboards: OverviewDashboard<L>[] = [];
    const existsIds: Record<string, boolean> = {};

    for (const item of source) {
        if (dashboards.length >= OVERVIEW_MAX_EXTRA_DASHBOARDS) {
            break;
        }

        if (!isObject(item)) {
            continue;
        }

        const { id, name, layout } = item as Record<string, unknown>;

        if (!isString(id) || !id || id.length > 100 || id === OVERVIEW_HOME_DASHBOARD_ID || existsIds[id]) {
            continue;
        }

        const trimmedName = isString(name) ? name.trim().substring(0, OVERVIEW_DASHBOARD_NAME_MAX_LENGTH).trim() : '';

        if (!trimmedName) {
            continue;
        }

        try {
            dashboards.push({ id, name: trimmedName, layout: normalize(layout) });
            existsIds[id] = true;
        } catch {
            continue;
        }
    }

    return dashboards;
}

function serializeOverviewExtraDashboards<L extends OverviewLayoutBase>(dashboards: OverviewDashboard<L>[], normalize: (input: unknown) => L): string {
    if (!dashboards.length) {
        return '';
    }

    return JSON.stringify(dashboards.map(dashboard => ({
        id: dashboard.id,
        name: dashboard.name,
        layout: normalize(dashboard.layout)
    })));
}

export function parseDesktopOverviewExtraDashboards(value: string): OverviewDashboard<DesktopOverviewLayout>[] {
    return parseOverviewExtraDashboards(value, normalizeDesktopOverviewLayout);
}

export function serializeDesktopOverviewExtraDashboards(dashboards: OverviewDashboard<DesktopOverviewLayout>[]): string {
    return serializeOverviewExtraDashboards(dashboards, normalizeDesktopOverviewLayout);
}

export function parseMobileOverviewExtraDashboards(value: string): OverviewDashboard<MobileOverviewLayout>[] {
    return parseOverviewExtraDashboards(value, normalizeMobileOverviewLayout);
}

export function serializeMobileOverviewExtraDashboards(dashboards: OverviewDashboard<MobileOverviewLayout>[]): string {
    return serializeOverviewExtraDashboards(dashboards, normalizeMobileOverviewLayout);
}

export function resolveSelectedDashboardId(selectedId: string, extraDashboards: { id: string }[]): string {
    return extraDashboards.some(dashboard => dashboard.id === selectedId) ? selectedId : OVERVIEW_HOME_DASHBOARD_ID;
}
