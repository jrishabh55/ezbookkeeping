package forecast

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func dates(items []Item) []time.Time {
	r := make([]time.Time, len(items))
	for i, it := range items {
		r[i] = it.Date
	}
	return r
}

func TestExpand_MonthlyMultiDay(t *testing.T) {
	tpl := Template{Id: 1, Kind: KindExpense, Amount: 100, FrequencyType: FreqMonthly, Frequency: "1,15"}
	got := ExpandTemplate(tpl, D(2026, 11, 1), D(2027, 1, 1))
	assert.Equal(t, []time.Time{D(2026, 11, 1), D(2026, 11, 15), D(2026, 12, 1), D(2026, 12, 15)}, dates(got))
	assert.Equal(t, SourceTemplate, got[0].Source)
	assert.Equal(t, int64(1), got[0].SourceId)
}

func TestExpand_MonthlyDay31InNovember(t *testing.T) {
	tpl := Template{Kind: KindExpense, FrequencyType: FreqMonthly, Frequency: "31"}
	assert.Equal(t, []time.Time{D(2026, 11, 30)}, dates(ExpandTemplate(tpl, D(2026, 11, 1), D(2026, 12, 1))))
	tpl.Frequency = "-1"
	assert.Equal(t, []time.Time{D(2026, 11, 30)}, dates(ExpandTemplate(tpl, D(2026, 11, 1), D(2026, 12, 1))))
}

func TestExpand_EndDateStops(t *testing.T) {
	tpl := Template{Kind: KindExpense, FrequencyType: FreqMonthly, Frequency: "3", End: D(2026, 11, 30)}
	assert.Equal(t, []time.Time{D(2026, 11, 3)}, dates(ExpandTemplate(tpl, D(2026, 11, 1), D(2027, 2, 1))))
}

func TestExpand_WeeklyYearlyEveryN(t *testing.T) {
	weekly := Template{FrequencyType: FreqWeekly, Frequency: "1"} // Mondays
	assert.Equal(t, []time.Time{D(2026, 11, 2), D(2026, 11, 9)}, dates(ExpandTemplate(weekly, D(2026, 11, 1), D(2026, 11, 15))))
	yearly := Template{FrequencyType: FreqYearly, Frequency: "1115"}
	assert.Equal(t, []time.Time{D(2026, 11, 15)}, dates(ExpandTemplate(yearly, D(2026, 11, 1), D(2026, 12, 1))))
	everyN := Template{FrequencyType: FreqEveryNDay, Frequency: "10", Start: D(2026, 10, 28)}
	assert.Equal(t, []time.Time{D(2026, 11, 7), D(2026, 11, 17), D(2026, 11, 27)}, dates(ExpandTemplate(everyN, D(2026, 11, 1), D(2026, 12, 1))))
}

func TestExpand_InvalidFrequencyGivesNothing(t *testing.T) {
	assert.Empty(t, ExpandTemplate(Template{FrequencyType: FreqMonthly, Frequency: "x"}, D(2026, 11, 1), D(2026, 12, 1)))
	assert.Empty(t, ExpandTemplate(Template{FrequencyType: 0, Frequency: "1"}, D(2026, 11, 1), D(2026, 12, 1)))
}
