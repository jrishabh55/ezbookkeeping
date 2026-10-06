# SMS Auto-capture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bank SMS on the user's iPhone become categorised transactions in the right ezBookkeeping account within seconds, via an iOS Shortcut posting to a per-user, ingest-only endpoint; statement imports reconcile against them.

**Architecture:** A pure Go package `pkg/alerts` parses SMS text and classifies it (no DB), a service `pkg/services/alerts.go` loads the user's accounts/history, dedupes, creates the transaction and stores the raw message, and `pkg/api/alerts.go` exposes an ingest endpoint guarded by a new ingest-only token type. A settings page (mobile + desktop) issues the token and links a shared iCloud shortcut. The statement import flow marks rows already captured by SMS.

**Tech Stack:** Go 1.27 (gin, xorm, testify), Vue 3 + Vuetify (desktop) / Framework7 (mobile), iOS Shortcuts.

**Spec:** `docs/superpowers/specs/2026-10-06-sms-auto-capture-design.md`

## Global Constraints

- The user is taken from the token claims only; no request field selects or carries a user.
- New token type `USER_TOKEN_TYPE_ALERT_INGEST TokenType = 9`; accepted only on `POST /api/v1/alerts/ingest.json`; rejected by every other middleware (already true for unknown types in `jwtAuthorization`, keep it that way).
- At most one active ingest token per user; creating a new one deletes the previous ingest token records.
- Ingest body: `{"sender": string, "text": string, "receivedAt": int64 unix seconds}`; `text` capped at 2048 bytes; rate limit 30 requests/minute per user.
- Response `result` is exactly one of `added`, `duplicate`, `ignored`, `unparsed`; `summary` only for `added`; never balances or other transactions.
- Tags created per user on demand, exact names: `Auto (SMS)`, `Needs review`, `Balance mismatch`, `Statement verified`, `Not in statement`.
- Amounts are stored in the smallest unit (paise), like the rest of the app.
- Committed fixtures and code must not contain the user's real names, account numbers, references or amounts.
- Commits on branch `budgets-buckets` use `git commit --no-verify` (the repo's stale `bd` pre-commit hook fails on every commit).

## Review Focus

1. Same SMS delivered twice (iOS automations can fire twice): second POST must return `duplicate`, not a second transaction (Task 6 test `TestIngestDuplicateReference`).
2. OTP / promotional / payment-due SMS that contain "Rs": must return `ignored` and create nothing (Task 2 table cases `otp`, `promo`, `due-reminder`).
3. Last-4 that matches no account of this user, but matches another user's account: must land in this user's "Unmatched alerts" account, never touch the other user (Task 6 test `TestIngestNeverResolvesOtherUsersAccount`).
4. Indian number formats `1,23,456.78`, `Rs 500`, `INR500.5`: amount parsed exactly (Task 2 table cases `lakh-format`, `no-decimals`, `one-decimal`).
5. Transfer between own accounts arriving as debit SMS on A then credit SMS on B: one transfer, second SMS `duplicate` (Task 6 test `TestIngestOwnTransferPair`).

---

## File Structure

| File | Responsibility |
|---|---|
| `pkg/alerts/parser.go` | `ParsedAlert`, `Parse()`: SMS text → structured alert or ignored/unparsed |
| `pkg/alerts/parser_test.go` | table tests + golden fixture test |
| `pkg/alerts/testdata/sms_fixtures.json` | anonymised real SMS + expected parse results |
| `pkg/alerts/payee.go` | `PayeeKey()`: counterparty key from SMS counterparty or statement narration |
| `pkg/alerts/classify.go` | `Classify()`: own-account transfer / history / keywords / fallback (pure) |
| `pkg/alerts/classify_test.go` | classifier + payee key tests |
| `pkg/core/token_claims.go` | add token type 9 |
| `pkg/services/tokens.go` | create/revoke ingest token, list includes type 9 |
| `pkg/middlewares/authorization.go` | `JWTAlertIngestAuthorization` |
| `pkg/models/alert_message.go` | `AlertMessage` table + request/response models |
| `pkg/uuid/uuid_type.go` | `UUID_TYPE_ALERT_MESSAGE UuidType = 14` |
| `cmd/database.go` | sync `AlertMessage` |
| `pkg/services/alerts.go` | `AlertService.Ingest()` orchestration, dedupe, tags, balance check |
| `pkg/services/alerts_test.go` | service tests on the test datastore |
| `pkg/api/alerts.go` | ingest handler, setup endpoints (token, status) |
| `cmd/webserver.go` | routes |
| `pkg/settings/setting.go`, `conf/ezbookkeeping.ini` | `[alerts] shortcut_url` |
| `src/lib/services.ts`, `src/models/alert.ts` | frontend API |
| `src/views/mobile/settings/SmsCapturePage.vue`, `src/router/mobile.ts`, `src/views/mobile/SettingsPage.vue` | mobile setup page |
| `src/views/desktop/settings/SmsCapturePage.vue`, `src/router/desktop.ts`, `src/views/desktop/settings/SettingsPageLayout.vue` | desktop setup page |
| `src/locales/en.json` | strings |
| `pkg/api/transactions.go`, `pkg/models/imported_transaction.go`, `src/views/desktop/transactions/import/*` | statement reconciliation |
| `docs/sms-shortcut.md` | how the shared shortcut is built (for re-creating it) |

---

### Task 1: Collect anonymised SMS fixtures

Needs Full Disk Access for the terminal app (System Settings → Privacy & Security → Full Disk Access), then a restart of that app.

**Files:**
- Create: `scripts/sms_fixtures.py` (one-off helper, committed so fixtures can be refreshed)
- Create: `pkg/alerts/testdata/sms_fixtures.json`

**Interfaces:**
- Produces: `pkg/alerts/testdata/sms_fixtures.json`, a JSON array of `{"id": str, "sender": str, "text": str, "expected": null}`; Task 2 fills `expected`.

- [ ] **Step 1: Write the export script**

```python
#!/usr/bin/env python3
"""Export bank SMS from macOS Messages into anonymised fixtures.
Usage: python3 scripts/sms_fixtures.py > pkg/alerts/testdata/sms_fixtures.json
Reads ~/Library/Messages/chat.db read-only. Only senders that look like Indian bank / card IDs."""
import json, os, random, re, sqlite3

BANK_SENDER = re.compile(r'(HDFC|YESB|YESBNK|PNB|ICICI|AXIS|SBI|KOTAK|IDFC|RBL|BOB|DCB|CRED|AMEX|SCB|INDUS|AUBANK|ONECARD)', re.I)
DB = os.path.expanduser('~/Library/Messages/chat.db')
rng = random.Random(7)

def fake_digits(m):
    return ''.join(str(rng.randint(0, 9)) for _ in m.group(0))

def anonymise(text):
    t = re.sub(r'(?<=[Rs\.INR ]{1})[\d,]+\.\d{1,2}', lambda m: f'{rng.randint(1, 99999):,}.{rng.randint(0, 99):02d}', text)  # amounts
    t = re.sub(r'\d{5,}', fake_digits, t)                                   # references, account numbers, phones
    t = re.sub(r'(?<=[Xx*]{2})\d{4}|(?<=[Xx*]{4})\d{4}|(?<=\*)\d{4}', lambda m: '1234', t)  # masked last-4
    t = re.sub(r'[A-Za-z0-9._-]+@[A-Za-z]+', 'payee@upi', t)                # VPAs
    t = re.sub(r'\b(To|to|At|at|from|From|by|By)\s+([A-Z][A-Za-z.&\' ]{2,40}?)(?=\s+(?:On|on|Ref|ref|UPI|via|\.|,)|$)',
               lambda m: m.group(1) + ' ' + rng.choice(['ACME STORE', 'JOHN DOE', 'FOOD CORNER', 'CITY FUELS']), t)
    return t

con = sqlite3.connect(f'file:{DB}?mode=ro', uri=True)
rows = con.execute("""SELECT h.id, m.text FROM message m JOIN handle h ON m.handle_id = h.ROWID
                      WHERE m.is_from_me = 0 AND m.text IS NOT NULL ORDER BY m.date DESC""").fetchall()
seen, out = set(), []
for sender, text in rows:
    if not BANK_SENDER.search(sender or ''): continue
    shape = re.sub(r'\d', '9', text)[:60]          # one sample per message shape
    if shape in seen: continue
    seen.add(shape)
    out.append({'id': f'f{len(out) + 1:03d}', 'sender': re.sub(r'^[A-Z]{2}-', 'XX-', sender), 'text': anonymise(text), 'expected': None})
print(json.dumps(out, indent=1, ensure_ascii=False))
```

- [ ] **Step 2: Run it and check nothing real leaked**

Run: `mkdir -p pkg/alerts/testdata && python3 scripts/sms_fixtures.py > pkg/alerts/testdata/sms_fixtures.json && python3 -c "import json; d=json.load(open('pkg/alerts/testdata/sms_fixtures.json')); print(len(d), 'fixtures', sorted({x['sender'] for x in d}))"`
Expected: a few dozen fixtures, senders like `XX-HDFCBK`, `XX-YESBNK`, `XX-PNBSMS`.
Then grep for your real identifiers (account last-4s, your name, frequent payees), kept only in a local shell variable, and expect no output:
`grep -niE "$PRIVATE_IDS" pkg/alerts/testdata/sms_fixtures.json`  (e.g. `export PRIVATE_IDS="1111|2222|YOUR NAME"`, never committed)
If anything prints, extend `anonymise()` and rerun. Read every fixture once by eye before committing.

- [ ] **Step 3: Commit**

```bash
git add scripts/sms_fixtures.py pkg/alerts/testdata/sms_fixtures.json
git commit --no-verify -m "alerts: anonymised bank SMS fixtures"
```

---

### Task 2: SMS parser

**Files:**
- Create: `pkg/alerts/parser.go`
- Test: `pkg/alerts/parser_test.go`
- Modify: `pkg/alerts/testdata/sms_fixtures.json` (fill `expected`)

**Interfaces:**
- Consumes: fixtures from Task 1.
- Produces:
```go
type Outcome string
const (OutcomeParsed Outcome = "parsed"; OutcomeIgnored Outcome = "ignored"; OutcomeUnparsed Outcome = "unparsed")
type Direction string
const (Debit Direction = "debit"; Credit Direction = "credit")
type ParsedAlert struct {
    Outcome      Outcome
    Direction    Direction
    Amount       int64  // paise
    Last4        string // account or card last 4, "" if absent
    Counterparty string // VPA name / merchant / person, uppercase, "" if absent
    Reference    string // UPI ref / RRN / IMPS ref, "" if absent
    Balance      int64  // available balance in paise
    HasBalance   bool
}
func Parse(sender string, text string) ParsedAlert
```

- [ ] **Step 1: Write the failing table test**

```go
package alerts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name, sender, text string
		want                ParsedAlert
	}{
		{"upi-debit", "XX-HDFCBK", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nOn 05/10/26\nRef 512345678901\nNot You?\nCall 18002586161/SMS BLOCK UPI to 7308080808",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 25000, Last4: "1234", Counterparty: "FOOD CORNER", Reference: "512345678901"}},
		{"credit-with-balance", "XX-HDFCBK", "Update! INR 1,23,456.78 deposited in HDFC Bank A/c XX1234 on 05-OCT-26 for NEFT Cr-ACME LTD. Avl bal INR 2,00,000.00. Cheque deposits in A/C are subject to clearing",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Credit, Amount: 12345678, Last4: "1234", Counterparty: "ACME LTD", Balance: 20000000, HasBalance: true}},
		{"card-spend", "XX-HDFCBK", "Spent Rs.1,234 On HDFC Bank Card 5678 At CITY FUELS On 2026-10-05:12:34:56.Not You? To Block+Reissue Call 18002323232/SMS BLOCK CC 5678 to 7308080808",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 123400, Last4: "5678", Counterparty: "CITY FUELS"}},
		{"lakh-format", "XX-YESBNK", "INR 1,00,000.00 debited from A/c XX1234 on 05-10-2026 to JOHN DOE. Avl Bal INR 4.00",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 10000000, Last4: "1234", Counterparty: "JOHN DOE", Balance: 400, HasBalance: true}},
		{"no-decimals", "XX-PNBSMS", "Ac XX1234 debited Rs 500 dt 05-10-26 thru UPI ref no 512345678901. Bal Rs 137.51",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 50000, Last4: "1234", Reference: "512345678901", Balance: 13751, HasBalance: true}},
		{"one-decimal", "XX-PNBSMS", "Ac XX1234 credited INR500.5 dt 05-10-26. Bal INR 638.01",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Credit, Amount: 50050, Last4: "1234", Balance: 63801, HasBalance: true}},
		{"otp", "XX-HDFCBK", "OTP is 123456 for txn of Rs.250.00 at FOOD CORNER on HDFC Bank card ending 5678. Valid till 12:34. Do not share OTP",
			ParsedAlert{Outcome: OutcomeIgnored}},
		{"promo", "XX-HDFCBK", "Get Rs.5,000 cashback on your next EMI purchase! Apply now: hdfc.bank.in/x",
			ParsedAlert{Outcome: OutcomeIgnored}},
		{"due-reminder", "XX-HDFCBK", "Payment of Rs.17,512.00 for loan A/c XX1234 is due on 05-10-26. Please maintain sufficient balance",
			ParsedAlert{Outcome: OutcomeIgnored}},
		{"mandate", "XX-HDFCBK", "Your AutoPay mandate of Rs.500.00 for NETFLIX has been set up successfully",
			ParsedAlert{Outcome: OutcomeIgnored}},
		{"financial-but-unclear", "XX-HDFCBK", "Rs.500.00 transaction on A/c XX1234 could not be processed",
			ParsedAlert{Outcome: OutcomeUnparsed}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assert.Equal(t, c.want, Parse(c.sender, c.text)) })
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./pkg/alerts/ -run TestParse -v`
Expected: FAIL, `undefined: ParsedAlert` / `undefined: Parse`.

- [ ] **Step 3: Implement the parser**

```go
// Package alerts turns bank / card transaction SMS into structured alerts and classifies them.
package alerts

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Outcome string

const (
	OutcomeParsed   Outcome = "parsed"
	OutcomeIgnored  Outcome = "ignored"
	OutcomeUnparsed Outcome = "unparsed"
)

type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)

// ParsedAlert is one bank / card SMS reduced to the fields a transaction needs
type ParsedAlert struct {
	Outcome      Outcome
	Direction    Direction
	Amount       int64
	Last4        string
	Counterparty string
	Reference    string
	Balance      int64
	HasBalance   bool
}

var (
	reIgnore  = regexp.MustCompile(`(?i)\b(otp|one time password|cashback|offer|apply now|is due|due on|mandate|will be debited|request(ed)? (money|payment)|limit (has been )?(increased|enhanced)|statement (is|has been) generated)\b`)
	reAmount  = regexp.MustCompile(`(?i)(?:rs\.?|inr)\s*([\d,]+(?:\.\d{1,2})?)`)
	reDebit   = regexp.MustCompile(`(?i)\b(debited|sent|spent|withdrawn|paid|purchase|transferred to|dr\b)`)
	reCredit  = regexp.MustCompile(`(?i)\b(credited|deposited|received|refund(ed)?|cr\b)`)
	reLast4   = regexp.MustCompile(`(?i)(?:a/?c|acct|account|card)(?:\s+(?:no\.?|number|ending(?:\s+with)?))?\s*[:\-]?\s*(?:[x*]+\s*)?(\d{4})\b`)
	reRef     = regexp.MustCompile(`(?i)\b(?:ref(?:erence)?(?:\s*no\.?)?|rrn|upi(?:\s*ref)?|imps(?:\s*ref)?)\s*[:\-]?\s*(\d{6,})`)
	reBalance = regexp.MustCompile(`(?i)\b(?:avl\.?|avail(?:able)?)?\s*bal(?:ance)?\s*(?:is)?\s*[:\-]?\s*(?:rs\.?|inr)\s*([\d,]+(?:\.\d{1,2})?)`)
	reParty   = regexp.MustCompile(`(?i)(?:\bto\b|\bat\b|\bfor\s+(?:neft|imps|rtgs|upi)\s+cr-|\bfrom\b|\bby\b)\s*:?\s*([A-Z][A-Z0-9.&' ]{2,40}?)\s*(?:\n|\.(?:\s|$)| on\b| ref\b| via\b| upi\b|,|$)`)
)

// Parse reads one SMS; it never guesses: anything financial it cannot read fully is OutcomeUnparsed
func Parse(sender string, text string) ParsedAlert {
	if reIgnore.MatchString(text) {
		return ParsedAlert{Outcome: OutcomeIgnored}
	}

	amountMatch := reAmount.FindStringSubmatch(text)

	if amountMatch == nil {
		return ParsedAlert{Outcome: OutcomeIgnored}
	}

	isDebit, isCredit := reDebit.MatchString(text), reCredit.MatchString(text)

	if isDebit == isCredit {
		return ParsedAlert{Outcome: OutcomeUnparsed}
	}

	amount, ok := parseRupees(amountMatch[1])

	if !ok || amount <= 0 {
		return ParsedAlert{Outcome: OutcomeUnparsed}
	}

	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: amount}

	if isCredit {
		alert.Direction = Credit
	}

	if m := reLast4.FindStringSubmatch(text); m != nil {
		alert.Last4 = m[1]
	}

	if m := reRef.FindStringSubmatch(text); m != nil {
		alert.Reference = m[1]
	}

	if m := reBalance.FindStringSubmatch(text); m != nil && m[0] != amountMatch[0] {
		if b, ok := parseRupees(m[1]); ok {
			alert.Balance, alert.HasBalance = b, true
		}
	}

	if m := reParty.FindStringSubmatch(text); m != nil {
		alert.Counterparty = strings.ToUpper(strings.TrimSpace(m[1]))
	}

	if alert.Last4 == "" {
		return ParsedAlert{Outcome: OutcomeUnparsed}
	}

	return alert
}

// parseRupees reads "1,23,456.78" / "500" / "500.5" into paise
func parseRupees(s string) (int64, bool) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)

	if err != nil {
		return 0, false
	}

	return int64(math.Round(v * 100)), true
}
```

- [ ] **Step 4: Run the table test and iterate until it passes**

Run: `go test ./pkg/alerts/ -run TestParse -v`
Expected: PASS for all 11 cases. If one fails, adjust the matching regular expression for that case only and rerun the whole table (a fix must not break another row).

- [ ] **Step 5: Add the golden fixture test**

```go
func TestParseFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/sms_fixtures.json")
	assert.Nil(t, err)

	var fixtures []struct {
		ID       string       `json:"id"`
		Sender   string       `json:"sender"`
		Text     string       `json:"text"`
		Expected *ParsedAlert `json:"expected"`
	}
	assert.Nil(t, json.Unmarshal(data, &fixtures))

	for _, f := range fixtures {
		if assert.NotNil(t, f.Expected, "fixture %s has no expected result yet", f.ID) {
			assert.Equal(t, *f.Expected, Parse(f.Sender, f.Text), "fixture %s: %s", f.ID, f.Text)
		}
	}
}
```
(add `encoding/json` and `os` to the test imports)

- [ ] **Step 6: Fill `expected` for every fixture by review**

Run this helper, read each line against its SMS text, fix the parser where it is wrong (and rerun Step 4), then write the reviewed results into the file:
```bash
cat > /tmp/fill_expected_test.go <<'EOF'
package alerts
import ("encoding/json"; "os"; "testing")
func TestFillExpected(t *testing.T) {
	data, _ := os.ReadFile("testdata/sms_fixtures.json")
	var f []map[string]any
	json.Unmarshal(data, &f)
	for _, x := range f { p := Parse(x["sender"].(string), x["text"].(string)); x["expected"] = p; t.Logf("%s %+v | %s", x["id"], p, x["text"]) }
	out, _ := json.MarshalIndent(f, "", " ")
	os.WriteFile("testdata/sms_fixtures.json", out, 0644)
}
EOF
cp /tmp/fill_expected_test.go pkg/alerts/zz_fill_expected_test.go && go test ./pkg/alerts/ -run TestFillExpected -v; rm pkg/alerts/zz_fill_expected_test.go
```
Expected: every transaction SMS shows `Outcome:parsed` with the right amount, direction, last-4; every OTP / promo / reminder shows `ignored`. Only write the file after every line is correct.

- [ ] **Step 7: Run all parser tests**

Run: `go test ./pkg/alerts/ -v`
Expected: PASS (`TestParse`, `TestParseFixtures`).

- [ ] **Step 8: Commit**

```bash
git add pkg/alerts/parser.go pkg/alerts/parser_test.go pkg/alerts/testdata/sms_fixtures.json
git commit --no-verify -m "alerts: SMS parser with fixture tests"
```

---

### Task 3: Payee key and classifier (pure)

**Files:**
- Create: `pkg/alerts/payee.go`, `pkg/alerts/classify.go`
- Test: `pkg/alerts/classify_test.go`

**Interfaces:**
- Consumes: `ParsedAlert` (Task 2).
- Produces:
```go
func PayeeKey(text string) string // normalised counterparty from an SMS counterparty or a statement narration; "" if none
type OwnAccount struct { ID int64; Name string; Last4 string }
type HistoryHit struct { IsTransfer bool; CategoryId int64; OtherAccountId int64 }
type Classification struct {
    IsTransfer     bool
    CategoryId     int64 // 0 means "use the Other Expense/Other Income fallback"
    OtherAccountId int64 // transfer counterpart
    NeedsReview    bool
}
type KeywordRule struct { Pattern *regexp.Regexp; Direction Direction; CategoryId int64 }
func Classify(alert ParsedAlert, accountId int64, own []OwnAccount, history func(payeeKey string) (HistoryHit, bool), keywords []KeywordRule, textUpper string) Classification
```

- [ ] **Step 1: Write the failing tests**

```go
package alerts

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPayeeKey(t *testing.T) {
	assert.Equal(t, "JOHN DOE", PayeeKey("JOHN DOE"))
	assert.Equal(t, "JOHN DOE", PayeeKey("UPI-JOHN  DOE-JOHN99@OKAXIS-PUNB0000000-512345678901-PAYMENT"))
	assert.Equal(t, "JOHN DOE", PayeeKey("IMPS-512345678901-JOHN DOE-HDFC-XXXXXXXX1234-SALARY"))
	assert.Equal(t, "ACME LTD", PayeeKey("NEFT CR-ICIC0000001-ACME LTD-JOHN DOE-ICICN12345"))
	assert.Equal(t, "JOHN DOE", PayeeKey("50100000000000-TPT-SALARY-JOHN DOE"))
	assert.Equal(t, "", PayeeKey("INTEREST PAID TILL 31-MAR-2026"))
}

func TestClassifyOwnAccountByLast4(t *testing.T) {
	own := []OwnAccount{{ID: 1, Name: "HDFC Bank 1234", Last4: "1234"}, {ID: 2, Name: "Savings 5678", Last4: "5678"}}
	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "SELF"}
	c := Classify(alert, 1, own, noHistory, nil, "IMPS SENT FROM A/C XX1234 TO A/C XXXXXX5678")
	assert.Equal(t, Classification{IsTransfer: true, OtherAccountId: 2}, c)
}

func TestClassifyHistory(t *testing.T) {
	hist := func(k string) (HistoryHit, bool) {
		if k == "JOHN DOE" {
			return HistoryHit{CategoryId: 42}, true
		}
		return HistoryHit{}, false
	}
	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "JOHN DOE"}
	assert.Equal(t, Classification{CategoryId: 42}, Classify(alert, 1, nil, hist, nil, "SENT RS.1 TO JOHN DOE"))
}

func TestClassifyHistoryTransferTarget(t *testing.T) {
	hist := func(k string) (HistoryHit, bool) { return HistoryHit{IsTransfer: true, OtherAccountId: 9}, k == "CRED CLUB" }
	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "CRED CLUB"}
	assert.Equal(t, Classification{IsTransfer: true, OtherAccountId: 9}, Classify(alert, 1, nil, hist, nil, "SENT TO CRED CLUB"))
}

func TestClassifyKeywordThenFallback(t *testing.T) {
	kw := []KeywordRule{{Pattern: regexp.MustCompile(`ZOMATO|SWIGGY`), Direction: Debit, CategoryId: 7}}
	food := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "ZOMATO LTD"}
	assert.Equal(t, Classification{CategoryId: 7}, Classify(food, 1, nil, noHistory, kw, "SPENT AT ZOMATO LTD"))
	unknown := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "SOMEONE"}
	assert.Equal(t, Classification{NeedsReview: true}, Classify(unknown, 1, nil, noHistory, kw, "SENT TO SOMEONE"))
}

func noHistory(string) (HistoryHit, bool) { return HistoryHit{}, false }
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pkg/alerts/ -run 'TestPayeeKey|TestClassify' -v`
Expected: FAIL, `undefined: PayeeKey`.

- [ ] **Step 3: Implement `payee.go`**

```go
package alerts

import (
	"regexp"
	"strings"
)

var narrationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^UPI-([^-]+)-`),
	regexp.MustCompile(`^IMPS-\d+-([^-]+)-`),
	regexp.MustCompile(`^(?:NEFT|RTGS) (?:CR|DR)-[A-Z0-9]+-([^-]+)`),
	regexp.MustCompile(`^\d+-TPT-[^-]*-([^-]+)$`),
	regexp.MustCompile(`^ACH [CD]- ?([^-]+)`),
	regexp.MustCompile(`^(?:POS|ME DC SI) [\dX]+ ([A-Z .&*]+)`),
}

var reNotName = regexp.MustCompile(`\d|^(INTEREST|SWEEP|INT\.|MONTHLY|EMI|CC |ATW|NWD|SMS )`)

// PayeeKey normalises a counterparty so that the SMS form ("JOHN DOE") and the statement narration
// form ("UPI-JOHN  DOE-...") of the same payee produce the same key
func PayeeKey(text string) string {
	u := strings.ToUpper(strings.TrimSpace(text))

	for _, p := range narrationPatterns {
		if m := p.FindStringSubmatch(u); m != nil {
			u = m[1]
			break
		}
	}

	u = strings.Join(strings.Fields(u), " ")

	if u == "" || reNotName.MatchString(u) {
		return ""
	}

	return u
}
```

- [ ] **Step 4: Implement `classify.go`**

```go
package alerts

import (
	"regexp"
	"strings"
)

type OwnAccount struct {
	ID    int64
	Name  string
	Last4 string
}

type HistoryHit struct {
	IsTransfer     bool
	CategoryId     int64
	OtherAccountId int64
}

type Classification struct {
	IsTransfer     bool
	CategoryId     int64
	OtherAccountId int64
	NeedsReview    bool
}

type KeywordRule struct {
	Pattern    *regexp.Regexp
	Direction  Direction
	CategoryId int64
}

var reAnyLast4 = regexp.MustCompile(`(?:X|\*){2,}\s*(\d{4})\b`)

// Classify decides category or transfer target: own account by last-4, then payee history, then keywords, then review
func Classify(alert ParsedAlert, accountId int64, own []OwnAccount, history func(payeeKey string) (HistoryHit, bool), keywords []KeywordRule, textUpper string) Classification {
	for _, m := range reAnyLast4.FindAllStringSubmatch(textUpper, -1) {
		for _, a := range own {
			if a.Last4 == m[1] && a.ID != accountId {
				return Classification{IsTransfer: true, OtherAccountId: a.ID}
			}
		}
	}

	if key := PayeeKey(alert.Counterparty); key != "" {
		if hit, ok := history(key); ok {
			if hit.IsTransfer && hit.OtherAccountId != accountId {
				return Classification{IsTransfer: true, OtherAccountId: hit.OtherAccountId}
			}
			if !hit.IsTransfer {
				return Classification{CategoryId: hit.CategoryId}
			}
		}
	}

	for _, k := range keywords {
		if k.Direction == alert.Direction && k.Pattern.MatchString(strings.ToUpper(alert.Counterparty+" "+textUpper)) {
			return Classification{CategoryId: k.CategoryId}
		}
	}

	return Classification{NeedsReview: true}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./pkg/alerts/ -v`
Expected: PASS (parser + payee + classifier tests).

- [ ] **Step 6: Commit**

```bash
git add pkg/alerts/payee.go pkg/alerts/classify.go pkg/alerts/classify_test.go
git commit --no-verify -m "alerts: payee key and history-first classifier"
```

---

### Task 4: Ingest token type, token service, middleware

**Files:**
- Modify: `pkg/core/token_claims.go:22-31`
- Modify: `pkg/services/tokens.go` (add `CreateAlertIngestToken`, include type 9 in `GetAllUnexpiredNormalAndMCPTokensByUid` at line 64)
- Modify: `pkg/middlewares/authorization.go` (add `JWTAlertIngestAuthorization` after `JWTMCPAuthorization`)
- Test: `pkg/middlewares/authorization_alert_test.go`

**Interfaces:**
- Produces:
```go
const USER_TOKEN_TYPE_ALERT_INGEST TokenType = 9                                  // pkg/core
func (s *TokenService) CreateAlertIngestToken(c *core.WebContext, user *models.User, expiresInSeconds int64) (string, *core.UserTokenClaims, error) // deletes older ingest tokens of this user first
func IsAlertIngestToken(claims *core.UserTokenClaims) bool                       // pkg/middlewares
func JWTAlertIngestAuthorization(config *settings.Config) core.MiddlewareHandlerFunc
```

- [ ] **Step 1: Failing test for the token-type check**

```go
package middlewares

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
)

func TestIsAlertIngestToken(t *testing.T) {
	assert.True(t, IsAlertIngestToken(&core.UserTokenClaims{Type: core.USER_TOKEN_TYPE_ALERT_INGEST}))
	for _, other := range []core.TokenType{core.USER_TOKEN_TYPE_NORMAL, core.USER_TOKEN_TYPE_MCP, core.USER_TOKEN_TYPE_API, core.USER_TOKEN_TYPE_REQUIRE_2FA} {
		assert.False(t, IsAlertIngestToken(&core.UserTokenClaims{Type: other}))
	}
	assert.False(t, IsAlertIngestToken(nil))
}
```

- [ ] **Step 2: Run, expect FAIL** — `go test ./pkg/middlewares/ -run TestIsAlertIngestToken -v` → `undefined: core.USER_TOKEN_TYPE_ALERT_INGEST`.

- [ ] **Step 3: Implement**

`pkg/core/token_claims.go`, add after `USER_TOKEN_TYPE_API`:
```go
	USER_TOKEN_TYPE_ALERT_INGEST                   TokenType = 9
```

`pkg/middlewares/authorization.go`, add after `JWTMCPAuthorization`:
```go
// IsAlertIngestToken reports whether the claims belong to an SMS alert ingest token
func IsAlertIngestToken(claims *core.UserTokenClaims) bool {
	return claims != nil && claims.Type == core.USER_TOKEN_TYPE_ALERT_INGEST
}

// JWTAlertIngestAuthorization accepts only alert ingest tokens (used solely on the ingest route)
func JWTAlertIngestAuthorization(config *settings.Config) core.MiddlewareHandlerFunc {
	return func(c *core.WebContext) {
		claims, tokenContext, err := getTokenClaims(c, TOKEN_SOURCE_TYPE_HEADER)

		if err != nil {
			utils.PrintJsonErrorResult(c, err)
			return
		}

		if !IsAlertIngestToken(claims) {
			log.Warnf(c, "[authorization.JWTAlertIngestAuthorization] user \"uid:%d\" token type (%d) is not an alert ingest token", claims.Uid, claims.Type)
			utils.PrintJsonErrorResult(c, errs.ErrCurrentInvalidTokenType)
			return
		}

		c.SetTokenClaims(claims)
		c.SetTokenContext(tokenContext)
		c.Next()
	}
}
```

`pkg/services/tokens.go`, add after `CreateMCPToken`:
```go
// CreateAlertIngestToken replaces the user's SMS alert ingest token (one active token per user)
func (s *TokenService) CreateAlertIngestToken(c *core.WebContext, user *models.User, expiresInSeconds int64) (string, *core.UserTokenClaims, error) {
	var old []*models.TokenRecord

	if err := s.TokenDB(user.Uid).NewSession(c).Where("uid=? AND token_type=?", user.Uid, core.USER_TOKEN_TYPE_ALERT_INGEST).Find(&old); err != nil {
		return "", nil, err
	}

	if len(old) > 0 {
		if err := s.DeleteTokens(c, user.Uid, old); err != nil {
			return "", nil, err
		}
	}

	duration := time.Unix(tokenMaxExpiredAtUnixTime, 0).Sub(time.Now())

	if expiresInSeconds > 0 {
		duration = time.Duration(expiresInSeconds) * time.Second
	}

	token, claims, _, err := s.createToken(c, user, core.USER_TOKEN_TYPE_ALERT_INGEST, s.getUserAgent(c), "", duration)
	return token, claims, err
}
```
and in `GetAllUnexpiredNormalAndMCPTokensByUid` change the `Where` to also include `core.USER_TOKEN_TYPE_ALERT_INGEST`:
```go
	err := s.TokenDB(uid).NewSession(c).Cols("uid", "user_token_id", "token_type", "user_agent", "created_unix_time", "expired_unix_time", "last_seen_unix_time").Where("uid=? AND (token_type=? OR token_type=? OR token_type=? OR token_type=?) AND expired_unix_time>?", uid, core.USER_TOKEN_TYPE_NORMAL, core.USER_TOKEN_TYPE_MCP, core.USER_TOKEN_TYPE_API, core.USER_TOKEN_TYPE_ALERT_INGEST, now).Find(&tokenRecords)
```

- [ ] **Step 4: Run** `go test ./pkg/middlewares/ ./pkg/services/ -v -run 'TestIsAlertIngestToken'` and `go build ./...` → PASS / no errors.

- [ ] **Step 5: Commit**

```bash
git add pkg/core/token_claims.go pkg/services/tokens.go pkg/middlewares/authorization.go pkg/middlewares/authorization_alert_test.go
git commit --no-verify -m "alerts: ingest-only token type and middleware"
```

---

### Task 5: Alert message model and migration

**Files:**
- Create: `pkg/models/alert_message.go`
- Modify: `pkg/uuid/uuid_type.go` (add `UUID_TYPE_ALERT_MESSAGE UuidType = 14`)
- Modify: `cmd/database.go` (sync after the bucket transaction table, same pattern)

**Interfaces:**
- Produces:
```go
type AlertMessage struct {
    AlertId       int64  `xorm:"PK"`
    Uid           int64  `xorm:"INDEX(IDX_alert_message_uid_ref) INDEX(IDX_alert_message_uid_time) NOT NULL"`
    Sender        string `xorm:"VARCHAR(32) NOT NULL"`
    Text          string `xorm:"VARCHAR(2048) NOT NULL"`
    Reference     string `xorm:"INDEX(IDX_alert_message_uid_ref) VARCHAR(32) NOT NULL"`
    Outcome       string `xorm:"VARCHAR(16) NOT NULL"`
    TransactionId int64  `xorm:"NOT NULL DEFAULT 0"`
    ReceivedUnixTime int64 `xorm:"INDEX(IDX_alert_message_uid_time) NOT NULL"`
    CreatedUnixTime  int64
}
type AlertIngestRequest struct { Sender string `json:"sender" binding:"max=32"`; Text string `json:"text" binding:"required,max=2048"`; ReceivedAt int64 `json:"receivedAt" binding:"min=0"` }
type AlertIngestResponse struct { Result string `json:"result"`; Summary string `json:"summary,omitempty"` }
type AlertTokenCreateRequest struct { Password string `json:"password" binding:"required"` }
type AlertTokenCreateResponse struct { Token string `json:"token"`; ShortcutUrl string `json:"shortcutUrl"` }
type AlertStatusResponse struct { Configured bool `json:"configured"`; LastReceivedAt int64 `json:"lastReceivedAt"`; LastOutcome string `json:"lastOutcome"`; Counts map[string]int64 `json:"counts"`; ShortcutUrl string `json:"shortcutUrl"` }
```

- [ ] **Step 1: Create `pkg/models/alert_message.go`** with exactly the types above (package `models`, each type with a one-line doc comment).
- [ ] **Step 2: Add the uuid type** in `pkg/uuid/uuid_type.go` after `UUID_TYPE_BUCKET`: `UUID_TYPE_ALERT_MESSAGE UuidType = 14`.
- [ ] **Step 3: Add the migration** in `cmd/database.go`, after the `models.BucketTransaction` block:
```go
	err = datastore.Container.UserDataStore.SyncStructs(new(models.AlertMessage))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] alert message table maintained successfully")
```
- [ ] **Step 4: Verify** `go build ./... && go vet ./pkg/models/ ./cmd/` → no output; start the local test server from a copy of the dev DB and expect the log line `alert message table maintained successfully`.
- [ ] **Step 5: Commit** `git add pkg/models/alert_message.go pkg/uuid/uuid_type.go cmd/database.go && git commit --no-verify -m "alerts: alert message table"`

---

### Task 6: Alert service (ingest orchestration)

**Files:**
- Create: `pkg/services/alerts.go`
- Test: `pkg/services/alerts_test.go`

**Interfaces:**
- Consumes: `alerts.Parse`, `alerts.Classify`, `alerts.PayeeKey` (Tasks 2–3); `models.AlertMessage` (Task 5); `TransactionService.CreateTransaction(c, tx, tagIds, pictureIds)`, `TransactionService.GetTransactionsByMaxTime(...)`, `AccountService.GetAllAccountsByUid`, `TransactionTagService.GetAllTagsByUid` / `CreateTags(c, uid, tags, skipExists=true)`, `TransactionCategoryService.GetAllCategoriesByUid`.
- Produces:
```go
type AlertIngestResult struct { Outcome string; Summary string; TransactionId int64 }
func (s *AlertService) Ingest(c core.Context, uid int64, sender string, text string, receivedAt time.Time) (*AlertIngestResult, error)
func (s *AlertService) Status(c core.Context, uid int64) (*models.AlertStatusResponse, error)
var Alerts = &AlertService{...}
```

Behaviour (in this order, each step is one small private method):

1. `parsed := alerts.Parse(sender, text)`; store an `AlertMessage` row for every request (outcome filled at the end).
2. `ignored` → return `ignored`.
3. `unparsed` → return `unparsed` (row kept for review).
4. Duplicate by reference: `parsed.Reference != ""` and another `AlertMessage` of this `uid` with that reference and outcome `added` → `duplicate`.
5. Duplicate without reference: an `added` alert of this uid on the same account, same amount and direction, received within 120 s → `duplicate`.
6. Resolve account: among `GetAllAccountsByUid(uid)` (this user only), the account whose name ends with `parsed.Last4`; none → the user's account named `Unmatched alerts` (create it on first use: category checking, INR, balance 0) and force `NeedsReview`.
7. Own-transfer pair: if `parsed.Direction == Credit`, look for this user's transfer (type `TRANSFER_OUT`) whose `RelatedAccountId` is the resolved account, same amount, within 3 days before, tagged `Auto (SMS)` → `duplicate`.
8. Classify with `own` = this user's accounts with a 4-digit name suffix, `history` = search this user's transactions by keyword `PayeeKey(counterparty)` (`GetTransactionsByMaxTime` with `keyword`, count 20), first whose `PayeeKey(comment)` equals the key → `HistoryHit`, `keywords` = rules mapped to this user's category ids by sub-category name (`Food`, `Telephone Bill`, `Personal Car Expense`, `Subscriptions`, `Movies & Shows`).
9. Create the transaction (`TransactionTime = utils.GetMinTransactionTimeFromUnixTime(receivedAt.Unix())`, `TimezoneUtcOffset` 330, comment = the SMS text trimmed to 255) with tags `Auto (SMS)` (+ `Needs review` when flagged). Credits from own accounts become the transfer's destination side (source = other account).
10. Balance check: if `parsed.HasBalance` and the account balance after insert ≠ `parsed.Balance` → add tag `Balance mismatch`.
11. Update the `AlertMessage` row (outcome `added`, transaction id, reference) and return `added` with summary `"₹<amount> · <category name or 'Transfer to <account>'> · <account name>"`.

- [ ] **Step 1: Add a throwaway SQLite test database helper** (existing service tests are pure; these need a real database). Create `pkg/services/alerts_testutil_test.go`:

```go
package services

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

var alertTestDbReady bool

// newAlertTestUser creates a user with the given INR accounts in a temporary sqlite database
func newAlertTestUser(t *testing.T, accountNames ...string) (core.Context, int64) {
	t.Helper()
	ctx := core.NewNullContext()
	if !alertTestDbReady {
		cfg := &settings.Config{DatabaseConfig: &settings.DatabaseConfig{DatabaseType: settings.Sqlite3DbType,
			DatabasePath: filepath.Join(t.TempDir(), "alerts.db"), MaxIdleConnection: 2, ConnectionMaxLifeTime: 14400}, UuidGeneratorType: settings.InternalUuidGeneratorType}
		settings.SetCurrentConfig(cfg)
		if err := datastore.InitializeDataStore(cfg); err != nil { t.Fatal(err) }
		if err := uuid.InitializeUuidGenerator(cfg); err != nil { t.Fatal(err) }
		for _, m := range []any{new(models.User), new(models.Account), new(models.Transaction), new(models.TransactionCategory), new(models.TransactionTag),
			new(models.TransactionTagIndex), new(models.AlertMessage)} {
			if err := datastore.Container.UserDataStore.SyncStructs(m); err != nil { t.Fatal(err) }
		}
		alertTestDbReady = true
	}
	user := &models.User{Username: "u" + strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", ""), Email: "x@example.com", Nickname: "x", Password: "test123456", DefaultCurrency: "INR"}
	if err := Users.CreateUser(ctx, user, false); err != nil { t.Fatal(err) }
	for _, name := range accountNames {
		a := &models.Account{Uid: user.Uid, Name: name, Category: models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT, Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Icon: 1, Color: "000000", Currency: "INR"}
		if err := Accounts.CreateAccounts(ctx, a, 0, nil, nil, time.UTC); err != nil { t.Fatal(err) }
	}
	return ctx, user.Uid
}

func countTransactions(t *testing.T, ctx core.Context, uid int64) int {
	n, err := datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Where("uid=? AND deleted=? AND type<>?", uid, false, models.TRANSACTION_DB_TYPE_TRANSFER_IN).Count(&models.Transaction{})
	if err != nil { t.Fatal(err) }
	return int(n)
}

func accountNameOf(t *testing.T, ctx core.Context, uid int64, transactionId int64) string {
	tx, err := Transactions.GetTransactionByTransactionId(ctx, uid, transactionId)
	if err != nil { t.Fatal(err) }
	a, err := Accounts.GetAccountByAccountId(ctx, uid, tx.AccountId)
	if err != nil { t.Fatal(err) }
	return a.Name
}

func accountIdByName(t *testing.T, ctx core.Context, uid int64, name string) int64 {
	all, err := Accounts.GetAllAccountsByUid(ctx, uid)
	if err != nil { t.Fatal(err) }
	for _, a := range all { if a.Name == name { return a.AccountId } }
	t.Fatalf("no account %q", name); return 0
}

func tagNamesOf(t *testing.T, ctx core.Context, uid int64, transactionId int64) []string {
	idx, err := TransactionTags.GetAllTagIdsOfTransactions(ctx, uid, []int64{transactionId})
	if err != nil { t.Fatal(err) }
	tags, err := TransactionTags.GetTagsByTagIds(ctx, uid, idx[transactionId])
	if err != nil { t.Fatal(err) }
	names := []string{}
	for _, tg := range tags { names = append(names, tg.Name) }
	return names
}

func importRow(uid int64, accountName string, typ models.TransactionDbType, amount int64, at time.Time, narration string) *models.ImportTransaction {
	return &models.ImportTransaction{Transaction: &models.Transaction{Uid: uid, Type: typ, Amount: amount, TransactionTime: at.Unix() * 1000, Comment: narration},
		OriginalSourceAccountName: accountName}
}
```
Before writing it, confirm each referenced name exists (`grep -n "func (s \*TransactionService) GetTransactionByTransactionId\|func (s \*AccountService) GetAccountByAccountId\|func (s \*TransactionTagService) GetAllTagIdsOfTransactions\|func (s \*TransactionTagService) GetTagsByTagIds\|func SetCurrentConfig\|Sqlite3DbType\|InternalUuidGeneratorType" pkg -r`) and use the real names where they differ. Run `go test ./pkg/services/ -run XXX` to check it compiles.

- [ ] **Step 2: Write the failing service tests** in `pkg/services/alerts_test.go`

```go
func TestIngestDuplicateReference(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	sms := "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nOn 05/10/26\nRef 512345678901"
	r1, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", sms, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r1.Outcome)
	r2, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", sms, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "duplicate", r2.Outcome)
	assert.Equal(t, 1, countTransactions(t, ctx, uid))
}

func TestIngestNeverResolvesOtherUsersAccount(t *testing.T) {
	ctx, other := newAlertTestUser(t, "HDFC Bank 1234")
	_, uid := newAlertTestUser(t, "PNB 9999")
	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 512345678902", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r.Outcome)
	assert.Equal(t, 0, countTransactions(t, ctx, other))
	assert.Equal(t, "Unmatched alerts", accountNameOf(t, ctx, uid, r.TransactionId))
}

func TestIngestOwnTransferPair(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234", "Savings 5678")
	debit := "INR 5,000.00 debited from A/c XX1234 on 05-10-2026 IMPS to A/c XXXXXX5678 Ref 512345678903"
	credit := "INR 5,000.00 credited to A/c XX5678 on 05-10-2026 by IMPS from A/c XXXXXX1234"
	r1, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", debit, time.Now())
	r2, _ := Alerts.Ingest(ctx, uid, "XX-YESBNK", credit, time.Now())
	assert.Equal(t, "added", r1.Outcome)
	assert.Equal(t, "duplicate", r2.Outcome)
	assert.Equal(t, 1, countTransactions(t, ctx, uid))
}

func TestIngestIgnoresOtp(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "OTP is 123456 for txn of Rs.250.00 on card ending 1234", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "ignored", r.Outcome)
	assert.Equal(t, 0, countTransactions(t, ctx, uid))
}

func TestIngestBalanceMismatchTag(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "INR 100.00 debited from A/c XX1234 to JOHN DOE. Avl Bal INR 5,000.00", time.Now())
	assert.Contains(t, tagNamesOf(t, ctx, uid, r.TransactionId), "Balance mismatch") // account balance is -100.00, SMS says 5,000.00
}
```

- [ ] **Step 3: Run** `go test ./pkg/services/ -run TestIngest -v` → FAIL (`undefined: Alerts`).
- [ ] **Step 4: Implement `pkg/services/alerts.go`** following steps 1–11 above, one private method per step (`storeMessage`, `isDuplicateReference`, `isRecentDuplicate`, `resolveAccount`, `isOwnTransferCredit`, `classify`, `createTransaction`, `checkBalance`, `ensureTags`). Every query filters by `uid`.
- [ ] **Step 5: Run** `go test ./pkg/services/ -run TestIngest -v` → PASS (5 tests). Then `go test ./pkg/... ` → PASS.
- [ ] **Step 6: Commit** `git add pkg/services/alerts.go pkg/services/alerts_test.go pkg/services/alerts_testutil_test.go && git commit --no-verify -m "alerts: ingest service (dedupe, own-account pairing, history classification, balance check)"`

---

### Task 7: Alerts API, routes, config

**Files:**
- Create: `pkg/api/alerts.go`
- Modify: `cmd/webserver.go` (routes), `pkg/settings/setting.go` + `conf/ezbookkeeping.ini` (`[alerts] shortcut_url`)

**Interfaces:**
- Consumes: `Alerts.Ingest`, `Alerts.Status` (Task 6), `TokenService.CreateAlertIngestToken`, `JWTAlertIngestAuthorization` (Task 4), models (Task 5).
- Produces routes:
  - `POST /api/v1/alerts/ingest.json` (alert ingest token only) → `AlertIngestResponse`
  - `POST /api/v1/alerts/token.json` (normal session, password) → `AlertTokenCreateResponse`
  - `POST /api/v1/alerts/token/revoke.json` (normal session) → `true`
  - `GET /api/v1/alerts/status.json` (normal session) → `AlertStatusResponse`

- [ ] **Step 1: Config.** In `pkg/settings/setting.go` add `AlertsShortcutUrl string` to `Config` and load it in the section loader for `[alerts]` key `shortcut_url` (same pattern as `[mcp] enable_mcp`). Add to `conf/ezbookkeeping.ini`:
```ini
[alerts]
# iCloud link of the shared "Record bank SMS" shortcut shown on the SMS auto-capture settings page
shortcut_url =
```
- [ ] **Step 2: Ingest route group** in `cmd/webserver.go`, next to the `/mcp` group:
```go
	alertRoute := router.Group("/api/v1/alerts/ingest.json")
	alertRoute.Use(bindMiddleware(middlewares.RequestId(config), config))
	alertRoute.Use(bindMiddleware(middlewares.RequestLog, config))
	alertRoute.Use(bindMiddleware(middlewares.JWTAlertIngestAuthorization(config), config))
	alertRoute.POST("", bindApi(api.Alerts.IngestHandler, config))
```
and in the authenticated `apiV1Route` block:
```go
			apiV1Route.POST("/alerts/token.json", bindApi(api.Alerts.TokenCreateHandler, config))
			apiV1Route.POST("/alerts/token/revoke.json", bindApi(api.Alerts.TokenRevokeHandler, config))
			apiV1Route.GET("/alerts/status.json", bindApi(api.Alerts.StatusHandler, config))
```
- [ ] **Step 3: Handlers** in `pkg/api/alerts.go` (struct `AlertsApi` with `ApiUsingConfig`, singleton `Alerts`):
  - `IngestHandler`: bind `AlertIngestRequest`; reject `len(Text) > 2048`; rate limit via the existing duplicate checker: key `"alert-ingest:<uid>"`, max 30 per 60 s, over the limit → `errs.ErrTooManyRequests` (add this error if missing: HTTP 429); `receivedAt` 0 → now; call `Alerts.Ingest(c, c.GetCurrentUid(), …)`; return `AlertIngestResponse{Result, Summary}`.
  - `TokenCreateHandler`: require `claims.Type == USER_TOKEN_TYPE_NORMAL`, check password like `TokenGenerateMCPHandler`, call `CreateAlertIngestToken(c, user, 0)`, return token + `config.AlertsShortcutUrl`.
  - `TokenRevokeHandler`: delete this user's ingest token records.
  - `StatusHandler`: `Alerts.Status(c, uid)` plus `ShortcutUrl`.
- [ ] **Step 4: Verify on the local test build**, after `go build ./...` and restarting the test server:
```bash
B=http://127.0.0.1:8080/api
TOK=$(curl -s -H "Authorization: Bearer $SESSION" -H 'Content-Type: application/json' -d '{"password":"test123456"}' $B/v1/alerts/token.json | python3 -c 'import json,sys; print(json.load(sys.stdin)["result"]["token"])')
curl -s -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d '{"sender":"XX-HDFCBK","text":"Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 512345678909"}' $B/v1/alerts/ingest.json   # expect "result":"added"
curl -s -H "Authorization: Bearer $TOK" $B/v1/accounts/list.json      # expect error: current token type is invalid
curl -s -H "Authorization: Bearer $SESSION" -H 'Content-Type: application/json' -d '{"sender":"x","text":"Sent Rs.1"}' $B/v1/alerts/ingest.json  # expect error: token type invalid
```
- [ ] **Step 5: Commit** `git add pkg/api/alerts.go cmd/webserver.go pkg/settings/setting.go conf/ezbookkeeping.ini && git commit --no-verify -m "alerts: ingest + setup API"`

---

### Task 8: Setup page (mobile + desktop)

**Files:**
- Create: `src/models/alert.ts`, `src/views/mobile/settings/SmsCapturePage.vue`, `src/views/desktop/settings/SmsCapturePage.vue`
- Modify: `src/lib/services.ts`, `src/router/mobile.ts`, `src/router/desktop.ts`, `src/views/mobile/SettingsPage.vue`, `src/views/desktop/settings/SettingsPageLayout.vue`, `src/locales/en.json`

**Interfaces:**
- Consumes: the three setup routes (Task 7).
- Produces: route `/settings/sms_capture` (desktop and mobile), settings entry "SMS Auto-capture".

- [ ] **Step 1: API client** in `src/lib/services.ts`:
```ts
    createAlertToken: (req: { password: string }): ApiResponsePromise<AlertTokenCreateResponse> => axios.post<ApiResponse<AlertTokenCreateResponse>>('v1/alerts/token.json', req),
    revokeAlertToken: (): ApiResponsePromise<boolean> => axios.post<ApiResponse<boolean>>('v1/alerts/token/revoke.json', {}),
    getAlertStatus: (): ApiResponsePromise<AlertStatusResponse> => axios.get<ApiResponse<AlertStatusResponse>>('v1/alerts/status.json'),
```
with `src/models/alert.ts`:
```ts
export interface AlertTokenCreateResponse { readonly token: string; readonly shortcutUrl: string; }
export interface AlertStatusResponse { readonly configured: boolean; readonly lastReceivedAt: number; readonly lastOutcome: string; readonly counts: Record<string, number>; readonly shortcutUrl: string; }
```
- [ ] **Step 2: Mobile page** (Framework7, follow `src/views/mobile/SettingsPage.vue` list style; use `useI18nUIComponents()` for `showToast`/`showLoading`, never `f7.toast`): status line, "Set up" (password prompt → `createAlertToken` → show token in a read-only field with a Copy button), "Install shortcut" (opens `shortcutUrl`), the 4 automation steps as a numbered list, "Send test" (polls `getAlertStatus` every 3 s for 2 min and shows the newest outcome), "Revoke".
- [ ] **Step 3: Desktop page** (Vuetify card inside `SettingsPageLayout`, same actions; the desktop page is for revoking / status, setup is normally done on the phone).
- [ ] **Step 4: Routes and entries:** `/settings/sms_capture` in both routers; "SMS Auto-capture" item under Application Settings in `SettingsPageLayout.vue` and in the mobile settings list; strings in `en.json`.
- [ ] **Step 5: Verify:** `npx vue-tsc --noEmit && npx eslint src/views/mobile/settings/SmsCapturePage.vue src/views/desktop/settings/SmsCapturePage.vue src/lib/services.ts` → no output; run the local test build and drive the mobile page with the Playwright helper used before (`scratchpad/lib.js`): set up → token visible → no console errors.
- [ ] **Step 6: Commit** `git add src && git commit --no-verify -m "alerts: SMS auto-capture setup page"`

---

### Task 9: The shared iOS shortcut

**Files:**
- Create: `docs/sms-shortcut.md`

- [ ] **Step 1: Build the shortcut** "Record bank SMS" in the Shortcuts app on the iPhone:
  1. *Receive* Messages input from Share Sheet / automation.
  2. *Text* action → `Shortcut Input` (message body); *Get Details of Messages* → Sender.
  3. *Text* action holding the setup code (this field becomes the import question).
  4. *Get Contents of URL*: `https://books.codeation.io/api/v1/alerts/ingest.json`, Method POST, Headers `Authorization: Bearer <setup code>`, `Content-Type: application/json`, Request Body JSON `sender` = Sender, `text` = message body.
  5. *Get Dictionary Value* `success`; *If* not true → *Show Notification* "Couldn't record this SMS, it will be picked up from the statement".
- [ ] **Step 2: Share with an import question:** Share → Copy iCloud Link, with "Set Up Import Questions" asking "Paste your setup code" for the setup-code Text action. Check the shared copy contains no token.
- [ ] **Step 3: Configure the link:** set `EBK_ALERTS_SHORTCUT_URL=<icloud link>` in the Dokploy environment (same flow as the MCP variables).
- [ ] **Step 4: Write `docs/sms-shortcut.md`** with the action list above and the automation steps (Automation → New → Message → Message Contains "Rs" → Run Immediately → Run Shortcut "Record bank SMS").
- [ ] **Step 5: Commit** `git add docs/sms-shortcut.md && git commit --no-verify -m "docs: how the shared SMS shortcut is built"`

---

### Task 10: Statement reconciliation

**Files:**
- Modify: `pkg/models/imported_transaction.go` (`MatchedTransactionId int64 json:"matchedTransactionId,string,omitempty"` on `ImportTransactionResponse`)
- Modify: `pkg/api/transactions.go` (`TransactionParseImportFileHandler`, `TransactionImportHandler`)
- Modify: `pkg/services/alerts.go` (add `MatchStatementRows`, `MarkStatementVerified`, `MarkNotInStatement`)
- Modify: `src/views/desktop/transactions/import/ImportDialog.vue` (+ the review table component it uses)
- Test: `pkg/services/alerts_reconcile_test.go`

**Interfaces:**
- Produces:
```go
func (s *AlertService) MatchStatementRows(c core.Context, uid int64, rows []*models.ImportTransaction) (map[int]int64, error) // row index → matched SMS transaction id
func (s *AlertService) MarkStatementVerified(c core.Context, uid int64, matches map[int64]*models.ImportTransaction) error  // swap tags, take statement time + narration
func (s *AlertService) MarkNotInStatement(c core.Context, uid int64, accountId int64, from, to int64) error              // tag unmatched Auto (SMS) txns in range
```

- [ ] **Step 1: Failing tests**
```go
func TestReconcileMatchesByReferenceAndAmount(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 512345678910", time.Now())
	rows := []*models.ImportTransaction{importRow(uid, "HDFC Bank 1234", models.TRANSACTION_DB_TYPE_EXPENSE, 25000, time.Now(), "UPI-FOOD CORNER-FOOD@OKAXIS-HDFC0000001-512345678910-UPI")}
	m, err := Alerts.MatchStatementRows(ctx, uid, rows)
	assert.Nil(t, err)
	assert.Equal(t, map[int]int64{0: r.TransactionId}, m)
}

func TestReconcileDoesNotMatchDifferentAmount(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 512345678911", time.Now())
	rows := []*models.ImportTransaction{importRow(uid, "HDFC Bank 1234", models.TRANSACTION_DB_TYPE_EXPENSE, 25100, time.Now(), "UPI-FOOD CORNER-512345678911")}
	m, _ := Alerts.MatchStatementRows(ctx, uid, rows)
	assert.Empty(t, m)
}

func TestReconcileMarksNotInStatement(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.99.00\nFrom HDFC Bank A/C *1234\nTo JOHN DOE\nRef 512345678912", time.Now())
	acc := accountIdByName(t, ctx, uid, "HDFC Bank 1234")
	assert.Nil(t, Alerts.MarkNotInStatement(ctx, uid, acc, time.Now().Add(-time.Hour).Unix(), time.Now().Add(time.Hour).Unix()))
	assert.Contains(t, tagNamesOf(t, ctx, uid, r.TransactionId), "Not in statement")
}
```
- [ ] **Step 2: Run** `go test ./pkg/services/ -run TestReconcile -v` → FAIL.
- [ ] **Step 3: Implement** matching (same account, same db type, amount equal, |date| ≤ 2 days, reference equal when both have one (statement reference = any 12-digit number in the narration)); in `TransactionParseImportFileHandler` set `MatchedTransactionId` on matched rows; in `TransactionImportHandler` accept `matchedTransactionIds` alongside the rows, call `MarkStatementVerified` for them and `MarkNotInStatement` for each imported account over the imported date range.
- [ ] **Step 4: Frontend:** in the import review table show a chip "Already recorded from SMS" when `matchedTransactionId` is set and default its checkbox to unticked; send matched ids with the import request; after import show each touched account's book balance.
- [ ] **Step 5: Run** `go test ./pkg/... && npx vue-tsc --noEmit` → PASS, then on the local test build: ingest one SMS, import a statement containing it, expect the row unticked with the chip and, after import, the SMS transaction tagged "Statement verified".
- [ ] **Step 6: Commit** `git add pkg src && git commit --no-verify -m "alerts: reconcile statement imports with SMS-captured transactions"`

---

### Task 11: Ship

- [ ] **Step 1:** `go vet ./pkg/... ./cmd && go test ./pkg/... && npx vue-tsc --noEmit && npx vitest run` → all PASS.
- [ ] **Step 2:** `git push` (GHCR workflow builds the multi-arch image; wait for `gh run watch` to succeed).
- [ ] **Step 3:** Dokploy: set `EBK_ALERTS_SHORTCUT_URL`, deploy, purge Cloudflare cache for `/server_settings.js`.
- [ ] **Step 4:** On the iPhone: Settings → SMS Auto-capture → Set up → Install shortcut → add the automation → Send test; make one ₹1 UPI payment and confirm it appears in the right account within a minute.
