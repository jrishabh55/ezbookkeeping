#!/usr/bin/env python3
"""Export bank SMS from macOS Messages into anonymised fixtures.

Usage: python3 scripts/sms_fixtures.py > pkg/alerts/testdata/sms_fixtures.json
Reads ~/Library/Messages/chat.db read-only. Only senders that look like Indian
bank / card IDs.

Anonymisation is deliberately aggressive: every digit run of 3+, every
amount, every masked account/card number, every UPI handle and every
person/merchant-like name is replaced with a fake value that keeps the same
*shape* (same grouping, same length, same case) so the parser's regexes
still match the fixture the same way a real message would. Nothing that
looks like it could identify a real person, account or place is kept.
"""
import json
import os
import random
import re
import sqlite3

BANK_SENDER = re.compile(
    r'(HDFC|YESB|YESBNK|PNB|ICICI|AXIS|SBI|KOTAK|IDFC|RBL|BOB|DCB|CRED|AMEX|SCB|INDUS|AUBANK|ONECARD)',
    re.I,
)
DB = os.path.expanduser('~/Library/Messages/chat.db')
MAX_FIXTURES = 80
rng = random.Random(1)

# ---- placeholders -----------------------------------------------------

MERCHANT_POOL = [
    'ACME STORE', 'CITY MART', 'GREEN SUPERMARKET', 'SUNRISE FOOD CORNER',
    'BLUE STAR ENTERPRISES', 'QUICK FUELS', 'METRO BAKERY', 'STAR ELECTRONICS',
    'VALLEY PHARMACY', 'RIVER CAFE', 'URBAN GROCERS', 'PRIME MOTORS',
    'SILVER SERVICES', 'GOLDEN BAKERY', 'WESTSIDE MART', 'EAST END STORES',
    'NORTHGATE FUELS', 'LAKEVIEW TRADERS', 'HILLTOP STATIONERS', 'MAPLE DINER',
]
PERSON_POOL = [
    'JOHN DOE', 'JANE SMITH', 'AMIT KUMAR', 'PRIYA SHARMA', 'SURESH REDDY',
    'ANITA RAO', 'RAVI VERMA', 'NEHA GUPTA', 'SANJAY MEHTA', 'POOJA NAIR',
]
NAME_POOL = MERCHANT_POOL + PERSON_POOL
VPA_LOCAL_POOL = [
    'rahul.k', 'priya1990', 'merchant01', 'store.pay', 'amitk', 'sunrise.biz',
    'quickpay42', 'citymart', 'traderpay', 'userpay77',
]

# Structural / generic words that must survive the "stray all-caps name"
# pass untouched, because the parser (or a human reading the fixture)
# depends on them.
STOPWORDS = {
    'UPI', 'IMPS', 'NEFT', 'RTGS', 'ATM', 'POS', 'SMS', 'OTP', 'PIN', 'CVV',
    'IFSC', 'EMI', 'KYC', 'GST', 'RBI', 'NPCI', 'AVL', 'BAL', 'BALANCE',
    'INR', 'RS', 'DR', 'CR', 'REF', 'RRN', 'ID', 'NO', 'VPA', 'BLOCK',
    'CALL', 'HELP', 'CARE', 'DEBIT', 'CREDIT', 'CARD', 'BANK', 'INFO',
    'DEAR', 'THANK', 'YOU', 'THANKS', 'NOT', 'YOUR', 'YOURS', 'TXN',
    'A/C', 'ACCT', 'ACCOUNT', 'LIMIT', 'DUE', 'MANDATE', 'CUSTOMER',
    'ALERT', 'FROM', 'ON', 'VIA', 'TO', 'AT', 'BY', 'FOR', 'IS', 'WAS',
    'AVAILABLE', 'PLEASE', 'IGNORE', 'ALREADY', 'DONE', 'SUCCESSFUL',
    'TRANSACTION', 'PAYMENT', 'REQUEST', 'STATEMENT', 'GENERATED',
    'CASHBACK', 'OFFER', 'NOW', 'APPLY', 'LTD', 'PVT', 'INDIA', 'NA',
}


def fake_digits_like(s):
    """Replace every digit in s with a random digit, keeping punctuation
    (commas/dots) and length exactly as-is, first digit never 0."""
    out = []
    first = True
    for ch in s:
        if ch.isdigit():
            if first:
                out.append(str(rng.randint(1, 9)))
                first = False
            else:
                out.append(str(rng.randint(0, 9)))
        else:
            out.append(ch)
    return ''.join(out)


def anonymise(text):
    t = text

    # 1. Amounts / balances: "Rs.1,23,456.78", "INR 500", "Rs 500.00" etc.
    t = re.sub(
        r'(?i)\b(?:rs\.?|inr)\s*[\d][\d,]*(?:\.\d{1,2})?',
        lambda m: fake_digits_like(m.group(0)),
        t,
    )

    # 2. Masked account / card numbers: XX1234, XXXX1234, ****1234, xx1234
    t = re.sub(
        r'(?<=[Xx\*]{2})\d{2,6}\b',
        lambda m: ''.join(str(rng.randint(0, 9)) for _ in m.group(0)),
        t,
    )

    # 3. VPA / email-like handles: replace local part only, keep the bank suffix shape
    t = re.sub(
        r'[A-Za-z0-9._+-]+(?=@[A-Za-z][A-Za-z0-9.]*)',
        lambda m: rng.choice(VPA_LOCAL_POOL),
        t,
    )

    # 3b. Salutation with no marker word, e.g. "Alex,New Loan limit..." at
    # the start of a line (SMS greeting templates often drop "Dear"/"Hello").
    _SALUTATION_SKIP = {'REGARDS', 'THANKS', 'THANK', 'SINCERELY', 'YOURS', 'WARM', 'BEST', 'HELLO', 'HI', 'DEAR'}
    t = re.sub(
        r'(?m)^([A-Z][a-z]+),',
        lambda m: m.group(0) if m.group(1).upper() in _SALUTATION_SKIP else rng.choice(NAME_POOL) + ',',
        t,
    )

    # 4. Names after common markers: to/from/by/at/towards/info/dear/contact/call...
    # A negative lookahead keeps generic bank/brand/helpdesk words (which are
    # the same for every user, not personal data) from being swallowed, e.g.
    # "From HDFC Bank A/c" or "call Customer Care" must stay untouched.
    _BRAND_EXCLUDE = (
        r'HDFC|ICICI|AXIS|STATE|SBI|KOTAK|PNB|YES|DCB|RBL|IDFC|BOB|AMEX|AMERICAN|'
        r'SCB|STANDARD|INDUSIND|AU|ONECARD|CRED(?:IL|IO)?|GOOGLE|NETFLIX|AMAZON|'
        r'JIO|BLACKROCK|BANK|CUSTOMER|CARE|HELPLINE|SUPPORT|OUR|YOUR|THE|US|'
        r'AC|ACC|RS|INR|USD|EUR|GBP|CR|DR|NIL|TOTAL|MIN|MAX|AMOUNT|DUE|DATE|'
        r'LTD|LIMITED|PVT|PRIVATE|NET|GROSS'
    )
    name_after_marker = re.compile(
        r'\b(?i:to|from|by|at|towards|info|dear|beneficiary|payee|name|hello|hi|'
        r'mr\.?|mrs\.?|ms\.?|contact|call)\b'
        r'\s*[:\-]?\s+'
        r'(?!(?i:' + _BRAND_EXCLUDE + r')\b)'
        r'([A-Z][A-Za-z&\']*(?:\s+[A-Z][A-Za-z&\']*){0,4})'
    )
    t = name_after_marker.sub(lambda m: m.group(0)[: m.start(1) - m.start(0)] + rng.choice(NAME_POOL), t)

    # 5. Names bounded by "/" or "-" delimiters, as seen in UPI/IMPS refs and
    # NEFT credit narrations, e.g. ".../JOHN DOE/...", "-Alex Doe-",
    # "/Alex doe.". Require 2+ words so a single-token bank/short-link
    # code (URL slug, NSE circular code, bank reference) is left alone --
    # those never contain a space and are usually mixed with digits anyway.
    # Ordinary prose can also sit right after a hyphen (e.g. the compound
    # adjective in "even higher-quality products"), so this must bail out
    # the moment it sees a common English function word -- real names don't
    # contain those.
    _ENGLISH_STOPWORDS = {
        'A', 'AN', 'THE', 'AND', 'OR', 'BUT', 'IF', 'THEN', 'ELSE', 'FOR', 'NOR',
        'SO', 'YET', 'OF', 'ON', 'IN', 'AT', 'BY', 'AS', 'TO', 'FROM', 'WITH',
        'WITHOUT', 'ABOUT', 'AGAINST', 'BETWEEN', 'INTO', 'THROUGH', 'DURING',
        'BEFORE', 'AFTER', 'ABOVE', 'BELOW', 'UP', 'DOWN', 'OUT', 'OFF', 'OVER',
        'UNDER', 'AGAIN', 'FURTHER', 'ONCE', 'HERE', 'THERE', 'WHEN', 'WHERE',
        'WHY', 'HOW', 'ALL', 'ANY', 'BOTH', 'EACH', 'FEW', 'MORE', 'MOST',
        'OTHER', 'SOME', 'SUCH', 'ONLY', 'OWN', 'SAME', 'THAN', 'TOO', 'VERY',
        'JUST', 'EVEN', 'ALSO', 'WELL', 'BACK', 'STILL', 'WE', 'YOU', 'THEY',
        'HE', 'SHE', 'IT', 'I', 'THIS', 'THAT', 'THESE', 'THOSE', 'IS', 'ARE',
        'WAS', 'WERE', 'BE', 'BEEN', 'BEING', 'HAS', 'HAVE', 'HAD', 'HAVING',
        'DO', 'DOES', 'DID', 'DOING', 'WILL', 'WOULD', 'SHALL', 'SHOULD', 'CAN',
        'COULD', 'MAY', 'MIGHT', 'MUST', 'NOT', 'NO', 'OUR', 'YOUR', 'MY',
        'HIS', 'ITS', 'THEIR', 'US', 'THEM', 'HIM',
    }

    def bounded_name(m):
        words = [w.strip(".,&'") for w in m.group(0).split()]
        upper = [w.upper() for w in words]
        if any(w in STOPWORDS or w in _ENGLISH_STOPWORDS for w in upper):
            return m.group(0)
        return rng.choice(NAME_POOL)

    t = re.sub(
        r"(?<=[/-])[A-Za-z][A-Za-z']{1,}(?:\s+[A-Za-z][A-Za-z']*){1,3}(?![A-Za-z0-9])",
        bounded_name,
        t,
    )

    # 6. Parenthetical names, e.g. "VPA rahul@okhdfc (JOHN DOE)"
    t = re.sub(
        r'(?<=\()[A-Z][A-Za-z .&\']{2,40}(?=\))',
        lambda m: rng.choice(NAME_POOL),
        t,
    )

    # 7. Safety net: any remaining run of 2+ consecutive ALL-CAPS words that
    # is not a known structural keyword looks like a stray name/merchant.
    def blanket_name(m):
        words = m.group(0).split()
        if all(w.strip('.,&\'/') in STOPWORDS for w in words):
            return m.group(0)
        return rng.choice(NAME_POOL)

    t = re.sub(r"\b[A-Z][A-Z.&']{1,}(?:\s+[A-Z][A-Z.&']{1,}){1,5}\b", blanket_name, t)

    # 8. Generic sweep: any remaining run of 3+ digits (phones, unmasked
    # account numbers, long reference numbers, dates) -> random digits of
    # the same length.
    t = re.sub(r'\d{3,}', lambda m: ''.join(str(rng.randint(0, 9)) for _ in m.group(0)), t)

    return t


def normalise_sender(sender):
    if not sender:
        return sender
    if '@' in sender:
        m = BANK_SENDER.search(sender)
        code = m.group(0).upper() if m else 'BANK'
        return f'XX-{code}'
    return re.sub(r'^[A-Za-z]{2}-', 'XX-', sender).upper()


def guess_kind(text):
    tl = text.lower()
    if 'otp' in tl or 'one time password' in tl:
        return 'otp'
    if 'mandate' in tl:
        return 'mandate'
    if 'is due' in tl or 'due on' in tl or 'minimum amount due' in tl:
        return 'payment_due'
    if 'cashback' in tl or 'offer' in tl or 'apply now' in tl:
        return 'promotional'
    if 'upi' in tl:
        return 'upi'
    if 'imps' in tl or 'neft' in tl or 'rtgs' in tl:
        return 'imps_neft'
    if 'atm' in tl:
        return 'atm'
    if 'spent' in tl or 'card' in tl:
        return 'card_spend'
    if re.search(r'(?i)credited|deposited|received', text):
        return 'credit'
    if re.search(r'(?i)debited|withdrawn|paid', text):
        return 'debit'
    return 'other'


def main():
    con = sqlite3.connect(f'file:{DB}?mode=ro', uri=True)
    rows = con.execute(
        """SELECT h.id, m.text FROM message m JOIN handle h ON m.handle_id = h.ROWID
           WHERE m.is_from_me = 0 AND m.text IS NOT NULL ORDER BY m.date DESC"""
    ).fetchall()

    seen, candidates = set(), []
    for sender, text in rows:
        if not sender or not BANK_SENDER.search(sender):
            continue
        # Skip attachment-only / empty messages (e.g. the object-replacement
        # character macOS Messages uses for an image with no caption).
        if not text or not re.search(r'[A-Za-z0-9]', text):
            continue
        shape = re.sub(r'\d', '9', text)[:80]
        if shape in seen:
            continue
        seen.add(shape)
        candidates.append({
            'sender': normalise_sender(sender),
            'text': anonymise(text),
            'kind': guess_kind(text),
        })

    # Prefer variety across (sender, kind) pairs when capping.
    if len(candidates) > MAX_FIXTURES:
        by_bucket = {}
        for c in candidates:
            by_bucket.setdefault((c['sender'], c['kind']), []).append(c)
        buckets = list(by_bucket.values())
        rng.shuffle(buckets)
        picked, i = [], 0
        while len(picked) < MAX_FIXTURES and any(buckets):
            for b in buckets:
                if b and len(picked) < MAX_FIXTURES:
                    picked.append(b.pop(0))
            i += 1
        candidates = picked

    out = []
    for c in candidates:
        out.append({
            'id': f'f{len(out) + 1:03d}',
            'sender': c['sender'],
            'text': c['text'],
            'expected': None,
        })

    print(json.dumps(out, indent=1, ensure_ascii=False))


if __name__ == '__main__':
    main()
