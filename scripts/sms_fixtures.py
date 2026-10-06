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
    # Common SMS-template vocabulary that must survive verbatim -- it's the
    # same wording for every user, and swallowing it into a fake name
    # garbles the sentence (e.g. "DO NOT SHARE-PNB" -> "<NAME>-PNB").
    'DO', 'SECRET', 'CONFIDENTIAL', 'NEVER', 'SHARE', 'VALID', 'KINDLY',
    'DISCLOSE', 'ANYONE', 'MINS', 'MIN', 'MINUTES', 'ASKS', 'EVER', 'WITH',
    'OR', 'AND', 'OF', 'IF',
}


_MONTH_ABBR = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']


def _rand_num_width(width, lo, hi):
    """A random integer in [lo, hi], zero-padded/truncated to exactly `width` digits."""
    hi = min(hi, 10 ** width - 1)
    return str(rng.randint(lo, max(lo, hi))).zfill(width)


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

    # -1. VPA / email-like handles: replace local part only, keep the bank
    # suffix shape. This must run *before* URL protection below, otherwise
    # a dotted local part (e.g. "fastag.support@kotak.com") would itself get
    # matched and shielded as a fake "domain" by step 0, and the real
    # domain would then be replaced with an opaque placeholder that breaks
    # this pass's own lookahead (it needs to see real letters after "@").
    t = re.sub(
        r'[A-Za-z0-9._+-]+(?=@[A-Za-z][A-Za-z0-9.]*)',
        lambda m: rng.choice(VPA_LOCAL_POOL),
        t,
    )

    # 0. URLs: keep the domain, randomise the path, and shield the whole
    # thing (as an opaque placeholder) from every later pass below, so a
    # name/id sweep can't reach into the path and insert a space or eat the
    # domain.
    _url_placeholders = []

    def protect_url(m):
        domain, path = m.group(1), m.group(2) or ''
        if path:
            alphabet = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
            path = '/' + ''.join(rng.choice(alphabet) if ch.isalnum() else ch for ch in path[1:])
        token = f'\x00URL{len(_url_placeholders)}\x00'
        _url_placeholders.append(domain + path)
        return token

    t = re.sub(
        r'\b(?>((?:https?://)?(?:[A-Za-z0-9-]+\.)+[a-z]{2,}(?::\d+)?)(/\S*)?)(?!@)',
        protect_url,
        t,
    )

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

    # 2b. Dates and times: jitter every real date/time to a random *valid*
    # value in the same format (same separators, zero-padding, month-name
    # case) so neither the real day/month nor the real time-of-day survives
    # -- the generic 3+ digit sweep below misses these because each
    # component (DD, MM, HH, MM, SS) is only 1-2 digits on its own.
    def jitter_numeric_date(m):
        d, sep, mo, y = m.group(1), m.group(2), m.group(3), m.group(4)
        return f'{_rand_num_width(len(d), 1, 28)}{sep}{_rand_num_width(len(mo), 1, 12)}{sep}{_rand_num_width(len(y), 0, 10 ** len(y) - 1)}'

    t = re.sub(r'\b(\d{1,2})([-/])(\d{1,2})\2(\d{2}|\d{4})\b', jitter_numeric_date, t)

    def jitter_iso_date(m):
        y, sep, mo, d = m.group(1), m.group(2), m.group(3), m.group(4)
        return f'{_rand_num_width(len(y), 1000, 9999)}{sep}{_rand_num_width(len(mo), 1, 12)}{sep}{_rand_num_width(len(d), 1, 28)}'

    t = re.sub(r'\b(\d{4})(-)(\d{1,2})\2(\d{1,2})\b', jitter_iso_date, t)

    def jitter_month_name_date(m):
        d, sep, mon, y = m.group(1), m.group(2), m.group(3), m.group(4)
        choice = rng.choice(_MONTH_ABBR)
        if mon.isupper():
            nmon = choice
        elif mon.islower():
            nmon = choice.lower()
        else:
            nmon = choice.capitalize()
        return f'{_rand_num_width(len(d), 1, 28)}{sep}{nmon}{sep}{_rand_num_width(len(y), 0, 10 ** len(y) - 1)}'

    t = re.sub(r'\b(\d{1,2})([-/])([A-Za-z]{3})\2(\d{2}|\d{4})\b', jitter_month_name_date, t)

    # Bare DD-MM / DD/MM with no year (e.g. "On 13-02") -- the lookahead
    # keeps this from double-touching a DD-MM-YY(YY) date already jittered
    # above, and the value ranges keep it from firing on unrelated number
    # pairs like item counts.
    def jitter_bare_day_month(m):
        d, sep, mo = m.group(1), m.group(2), m.group(3)
        if not (1 <= int(d) <= 31 and 1 <= int(mo) <= 12):
            return m.group(0)
        return f'{_rand_num_width(len(d), 1, 28)}{sep}{_rand_num_width(len(mo), 1, 12)}'

    t = re.sub(r'(?<!\d[-/])\b(\d{1,2})([-/])(\d{1,2})\b(?![-/]\d)', jitter_bare_day_month, t)

    def jitter_time(m):
        hh, mm, ss = m.group(1), m.group(2), m.group(3)
        out = f'{_rand_num_width(len(hh), 0, 23)}:{_rand_num_width(len(mm), 0, 59)}'
        if ss is not None:
            out += f':{_rand_num_width(len(ss), 0, 59)}'
        return out

    t = re.sub(r'\b(\d{1,2}):(\d{2})(?::(\d{2}))?\b', jitter_time, t)

    # Dot-separated time, e.g. "13.51.58" (some bank templates use dots
    # instead of colons). Seconds are required so this can't collide with a
    # 2-decimal amount like "13.51", which only has one dot.
    def jitter_dot_time(m):
        hh, mm, ss = m.group(1), m.group(2), m.group(3)
        return f'{_rand_num_width(len(hh), 0, 23)}.{_rand_num_width(len(mm), 0, 59)}.{_rand_num_width(len(ss), 0, 59)}'

    t = re.sub(r'\b(\d{1,2})\.(\d{2})\.(\d{2})\b', jitter_dot_time, t)

    # 2c. Indian vehicle registrations (FASTag/toll SMS): [state][RTO][series][number],
    # e.g. "PB23Z8602" or "PB 23 Z 8602" -> replaced entirely with a
    # same-shape fake. Restricted to real state/UT codes so this can't also
    # fire on unrelated 2-letter+digits shapes like masked card prefixes
    # (XX1234) or abbreviations (SL 1570, ON 06).
    _VEH_STATE = (
        'AP|AR|AS|BR|CH|CG|DN|DD|DL|GA|GJ|HR|HP|JK|JH|KA|KL|LA|LD|MP|MH|'
        'MN|ML|MZ|NL|OD|OR|PY|PB|RJ|SK|TN|TG|TS|TR|UP|UK|UA|WB|AN'
    )

    def jitter_vehicle(m):
        state, sp1, rto, sp2, series, sp3, num = m.groups()
        r_state = ''.join(rng.choice('ABCDEFGHIJKLMNOPQRSTUVWXYZ') for _ in state)
        r_rto = ''.join(str(rng.randint(0, 9)) for _ in rto)
        r_series = ''.join(rng.choice('ABCDEFGHIJKLMNOPQRSTUVWXYZ') for _ in series) if series else ''
        r_num = ''.join(str(rng.randint(0, 9)) for _ in num)
        return f'{r_state}{sp1}{r_rto}{sp2}{r_series}{sp3}{r_num}'

    t = re.sub(
        r'\b(' + _VEH_STATE + r')(\s?)(\d{1,2})(\s?)([A-Z]{0,3})(\s?)(\d{1,4})\b',
        jitter_vehicle,
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

    # Repeated words allow a single uppercase letter ("N", "R", "K") so names
    # like "FERNS N GOLDEN BAKERY" / "R K TRADERS" are swallowed whole
    # instead of leaving the 1-char word stranded as a false word boundary.
    t = re.sub(r"\b[A-Z][A-Z.&']{1,}(?:\s+[A-Z][A-Z.&']*){1,5}\b", blanket_name, t)

    # 7b. Reference-style IDs: an all-letter token of 8+ chars right after a
    # label word (Mandate ID: XVPCZZUZQO), and any standalone token of 5+
    # chars mixing letters and digits (SI Hub ID: X4NT6Nzdv0, N2JZW2) -- both
    # look like opaque mandate/UMRN/reference codes, not real words, so they
    # are replaced char-class-by-char-class (letter->letter, digit->digit,
    # case preserved) rather than swept up as a "name".
    def _rand_like(s):
        out = []
        for ch in s:
            if ch.isdigit():
                out.append(str(rng.randint(0, 9)))
            elif ch.isupper():
                out.append(rng.choice('ABCDEFGHIJKLMNOPQRSTUVWXYZ'))
            elif ch.islower():
                out.append(rng.choice('abcdefghijklmnopqrstuvwxyz'))
            else:
                out.append(ch)
        return ''.join(out)

    t = re.sub(
        r'(?i)\b(?:id|ref|mandate|umrn|instructions|hub)\b\s*[:#]?\s*([A-Za-z]{8,})\b',
        lambda m: m.group(0)[: m.start(1) - m.start(0)] + _rand_like(m.group(1)),
        t,
    )

    _RESERVED_TOKEN = re.compile(r'(?i)^(?:rs|inr|usd|eur|gbp)\d|^[x*]{2,}\d+$')

    def _mixed_token(m):
        s = m.group(0)
        if not (any(c.isdigit() for c in s) and any(c.isalpha() for c in s)):
            return s
        if _RESERVED_TOKEN.match(s):
            return s
        return _rand_like(s)

    t = re.sub(r'\b[A-Za-z0-9]{5,}\b', _mixed_token, t)

    # 8. Generic sweep: any remaining run of 3+ digits (phones, unmasked
    # account numbers, long reference numbers, dates) -> random digits of
    # the same length.
    t = re.sub(r'\d{3,}', lambda m: ''.join(str(rng.randint(0, 9)) for _ in m.group(0)), t)

    # 9. Restore the (already-randomised) URLs shielded in step 0.
    for i, val in enumerate(_url_placeholders):
        t = t.replace(f'\x00URL{i}\x00', val)

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
