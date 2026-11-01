package forecast

import "sort"

const matchTolerance = 0.20

// DropFulfilled removes template items that a real transaction in the same month already covers.
// Each transaction fulfils at most one item; the closest date wins.
func DropFulfilled(items []Item, actual []Txn) []Item {
	used := map[int]bool{}
	var kept []Item
	for _, it := range items {
		if it.Source != SourceTemplate {
			kept = append(kept, it)
			continue
		}
		best, bestGap := -1, 0.0
		for i, tx := range actual {
			if used[i] || tx.Kind != it.Kind || tx.AccountId != it.AccountId || tx.CategoryId != it.CategoryId ||
				!MonthStart(tx.Date).Equal(MonthStart(it.Date)) || !within(tx.Amount, it.Amount) {
				continue
			}
			gap := tx.Date.Sub(it.Date).Abs().Hours()
			if best < 0 || gap < bestGap {
				best, bestGap = i, gap
			}
		}
		if best >= 0 {
			used[best] = true
			continue
		}
		kept = append(kept, it)
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Date.Before(kept[j].Date) })
	return kept
}

func within(actual, expected int64) bool {
	if expected == 0 {
		return actual == 0
	}
	diff := float64(actual-expected) / float64(expected)
	return diff >= -matchTolerance && diff <= matchTolerance
}
