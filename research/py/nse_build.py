#!/usr/bin/env python3
"""Build the committed point-in-time member lists and the NSE price series from the raw data nse_fetch.py downloaded.

Usage: nse_build.py [members] [prices]   (no argument = both)

  members  Walks back from today's official constituent lists through every NSE Indices press release that changed the
           Nifty 50 or the Nifty Midcap 150, undoing each change, and checks that every step lands on exactly 50 or 150
           members and that every name a release adds is present (and every name it drops is absent) at that point.
           It then compares the result with every archived copy of NSE's own list (nse_fetch.py anchors).
           Hand-checked inputs live in research/nse/members_overrides.json (each entry cites its release).
           Writes internal/data/<index>_members.csv (one row per membership spell) and research/nse/index_changes.csv.
  prices   Stitches every member's official bhavcopy bars across NSE symbol changes, back-adjusts them for splits,
           bonuses, consolidations, rights issues and demergers from NSE's corporate-action records, and collects cash
           dividends, then audits every large daily move left (see audit_prices). Writes
           research/nse/corporate_actions.csv and research/nse/price_audit.csv (committed) and
           .research-data/nse/prices/<SYMBOL>.csv plus <SYMBOL>_div.csv (not committed; rebuilt from the raw data).

Symbols in every output are canonical: the company's latest NSE symbol (SESAGOA and SSLT are VEDL), so a rename never
splits a series or drops a member. Companies that later merged or delisted keep their last symbol (HDFC, SATYAMCOMP).
"""
import csv
import datetime as dt
import gzip
import io
import json
import os
import re
import sys
import zipfile
from collections import defaultdict

RAW = os.environ.get("NSE_OUT", ".research-data/nse")
OVERRIDES = "research/nse/members_overrides.json"
CHANGES_OUT = "research/nse/index_changes.csv"
CA_OUT = "research/nse/corporate_actions.csv"
CA_OVERRIDES = "research/nse/corporate_actions_overrides.json"

INDICES = {
    # index id: (size, constituent list file, first day of the reconstruction, heading pattern in press releases)
    "nifty50": (50, "ind_nifty50list.csv", "2008-01-01",
                re.compile(r"^(s\s*&\s*p\s+)?(cnx\s+nifty|nifty\s*50)(\s+index)?$", re.I)),
    "niftymidcap150": (150, "ind_niftymidcap150list.csv", "2016-09-30",
                       re.compile(r"^nifty\s+midcap\s*150(\s+index)?$", re.I)),
}


EARLIEST = dt.date(2007, 12, 1)  # releases taking effect before this cannot change a reconstruction starting in 2008


def d(s):
    return dt.date.fromisoformat(s)


# ---------------------------------------------------------------- symbol changes

class Symbols:
    """Date-aware map from the symbol a stock traded under on a day to its canonical (latest) NSE symbol."""

    def __init__(self, overrides):
        self.changes = defaultdict(list)  # old symbol -> [(effective date, new symbol)]
        with open(os.path.join(RAW, "lists", "symbolchange.csv"), newline="", encoding="latin-1") as f:
            for row in csv.reader(f):
                if len(row) < 4 or not row[1].strip() or row[1].strip() == "SM_KEY_SYMBOL":
                    continue
                try:
                    when = dt.datetime.strptime(row[3].strip(), "%d-%b-%Y").date()
                except ValueError:
                    continue
                self.changes[row[1].strip()].append((when, row[2].strip()))
        for old, new, when, _why in overrides.get("symbol_changes", []):
            self.changes[old].append((d(when), new))
        for v in self.changes.values():
            v.sort()

    def canonical(self, sym, on):
        """The symbol `sym` (as traded on date `on`) carries today. Follows chains such as SESAGOA -> SSLT -> VEDL."""
        for _ in range(20):
            nxt = [(w, n) for w, n in self.changes.get(sym, []) if w > on]
            if not nxt:
                return sym
            on, sym = nxt[0]
        raise ValueError(f"symbol change loop at {sym}")

    def history(self, canon):
        """Every (symbol, from, to) a canonical symbol traded under; `to` is exclusive, None = open."""
        back = defaultdict(list)
        for old, v in self.changes.items():
            for w, new in v:
                back[new].append((w, old))
        spans, sym, until = [], canon, None
        for _ in range(20):
            prev = sorted((w, o) for w, o in back.get(sym, []) if until is None or w <= until)
            # The rename into `sym` that happened last before `until` (symbols can be reused by other companies).
            prev = [(w, o) for w, o in prev if self.canonical(o, w - dt.timedelta(days=1)) == canon]
            if not prev:
                spans.append((sym, None, until))
                break
            w, o = prev[-1]
            spans.append((sym, w, until))
            sym, until = o, w
        return spans


# ---------------------------------------------------------------- press releases

MONTH = r"(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\.?"
DATE = re.compile(MONTH + r"\s+(\d{1,2})(?:st|nd|rd|th)?\s*,?\s*(\d{4})", re.I)
# A section heading is numbered ("(1) S&P CNX Nifty Index", "a) Nifty 50", "5)  Nifty Midcap 150 Index"); an unnumbered
# index name is prose ("...on account of their inclusion in\n Nifty 50 index").
HEAD = re.compile(r"^\s*\(?(?:\d{1,2}|[a-zA-Z]|[ivx]+)[\).]\s*((?:S\s*&\s*P\s+)?(?:CNX|NIFTY|Nifty|S&P)[^:]{0,70}?)\s*[:.]?\s*$")
ROW = re.compile(r"^\s*(\d{1,3})\.?\s+(\S.*?)\s*$")
SYM = re.compile(r"^[A-Z0-9][A-Z0-9&\-]{1,19}$")


def parse_date(m):
    mon = ["jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"].index(m.group(1)[:3].lower()) + 1
    return dt.date(int(m.group(3)), mon, int(m.group(2)))


def effective_date(title, text):
    m = re.search(r"w\.?\s?e\.?\s?f\.?\s*(.*)", title, re.I)
    if m and DATE.search(m.group(1)):
        return parse_date(DATE.search(m.group(1)))
    m = re.search(r"effective\s+(?:from\s+)?(?:the\s+)?(?:close\s+of\s+)?" + DATE.pattern, text, re.I | re.S)
    if m:
        return parse_date(DATE.search(m.group(0)))
    return None


def parse_release(text):
    """Yield (index id, 'in'|'out', company name, symbol or '') from the standard exclusion/inclusion tables."""
    cur, mode = None, None
    for line in text.replace("\f", "\n").splitlines():
        s = line.strip()
        if not s:
            continue
        low = s.lower()
        m = HEAD.match(line)
        if m and "following" not in low and len(s) < 90 and not ROW.match(line):
            h = re.sub(r"\s+", " ", m.group(1)).strip(" .:")
            cur = next((k for k, v in INDICES.items() if v[3].match(h)), None)
            mode = None
            continue
        if cur is None:
            continue
        if "following" in low and ("exclu" in low or "includ" in low):
            mode = "out" if "exclu" in low else "in"
            continue
        if re.match(r"^(note|the above|about nse|about national)", low):
            mode = None
            continue
        if mode:
            r = ROW.match(line)
            if r:
                parts = re.split(r"\s{2,}", r.group(2))
                sym = parts[-1] if len(parts) > 1 and SYM.match(parts[-1]) else ""
                yield cur, mode, " ".join(parts[0].split()), sym


def load_releases(ov):
    with open(os.path.join(RAW, "press", "_index.json")) as f:
        index = json.load(f)
    rels = {}
    for r in index:
        if r["wanted"]:
            rels[r["file"][:-4]] = r
    events = []  # dicts: index, date, action, symbol (as named in the release), name, release
    unnamed = []
    for rid, r in sorted(rels.items(), key=lambda kv: kv[1]["date"]):
        if rid in ov["skip_releases"]:
            continue
        path = os.path.join(RAW, "press", rid + ".txt")
        if not os.path.exists(path):
            if os.path.exists(os.path.join(RAW, "press", rid + ".pdf.notpdf")):
                continue
            sys.exit(f"members: {rid} is missing; run research/py/nse_fetch.py press")
        text = open(path, encoding="utf-8", errors="replace").read()
        if len(text.strip()) < 300 and rid not in ov["image_only"]:
            # A scanned release has no text layer; it must be read by eye (or OCR) and recorded in the overrides.
            sys.exit(f"members: {rid} ({r['title']}) has no text layer; read it and record what it changes in "
                     f"{OVERRIDES} image_only")
        rows = list(parse_release(text))
        if not rows:
            continue
        when = ov["effective_dates"].get(rid)
        when = d(when) if when else effective_date(r["title"], text)
        if when is not None and when < EARLIEST:
            continue
        if when is None:
            sys.exit(f"members: no effective date found in {rid} ({r['title']}); add it to {OVERRIDES} effective_dates")
        for idx, action, name, sym in rows:
            key = f"{rid}:{name}"
            sym = ov["names"].get(key) or sym or ov["names"].get(name, "")
            if not sym:
                unnamed.append(f"{rid} {idx} {action} {name!r}")
                continue
            if sym.startswith("DUMMY"):
                continue  # temporary demerger placeholders (see overrides _comment)
            events.append({"index": idx, "date": when, "action": action, "symbol": sym, "name": name, "release": rid})
    if unnamed:
        sys.exit("members: these release rows name a company without a symbol; map them in "
                 f"{OVERRIDES} names:\n  " + "\n  ".join(unnamed))
    for e in ov["extra_events"]:
        events.append({"index": e["index"], "date": d(e["date"]), "action": e["action"], "symbol": e["symbol"],
                       "name": e.get("name", ""), "release": e["release"]})
    # A drop entry without "symbol" voids every change that release made to that index.
    drop = {(e["release"], e["index"], e.get("symbol", "*")) for e in ov["drop_events"]}
    events = [e for e in events if not {(e["release"], e["index"], e["symbol"]), (e["release"], e["index"], "*")} & drop]
    return events


# ---------------------------------------------------------------- membership walk

def read_list(name):
    with open(os.path.join(RAW, "lists", name), newline="") as f:
        return sorted({row["Symbol"].strip() for row in csv.DictReader(f)})


def expected_size(ov, idx, size, on):
    """The index's member count on a day: its nominal size unless an override records NSE carrying an extra security."""
    for x in ov["size_exceptions"]:
        if x["index"] == idx and d(x["from"]) <= on < d(x["to"]):
            return x["size"]
    return size


def build_members():
    ov = json.load(open(OVERRIDES))
    for k in ("skip_releases", "effective_dates", "names", "extra_events", "drop_events", "symbol_changes", "size_exceptions",
              "image_only"):
        ov.setdefault(k, {} if k in ("skip_releases", "effective_dates", "names", "image_only") else [])
    syms = Symbols(ov)
    fetched = d(open(os.path.join(RAW, "lists", "_fetched.txt")).read().strip())
    events = load_releases(ov)
    for e in events:
        e["canon"] = syms.canonical(e["symbol"], e["date"] - dt.timedelta(days=1))
    errors, all_rows = [], []
    for idx, (size, listfile, first, _) in INDICES.items():
        cur = {syms.canonical(s, fetched): None for s in read_list(listfile)}  # canon -> spell end (None = open)
        if len(cur) != expected_size(ov, idx, size, fetched):
            errors.append(f"{idx}: today's list has {len(cur)} members, want {size}")
        spells = []  # (canon, from, to, added_by, removed_by)
        removed_by = {s: "" for s in cur}
        evs = sorted((e for e in events if e["index"] == idx and e["date"] <= fetched), key=lambda e: e["date"], reverse=True)
        by_date = defaultdict(list)
        for e in evs:
            by_date[e["date"]].append(e)
        for when in sorted(by_date, reverse=True):
            if when <= d(first):
                break
            group = by_date[when]
            ins = {e["canon"]: e for e in group if e["action"] == "in"}
            outs = {e["canon"]: e for e in group if e["action"] == "out"}
            for c in set(ins) & set(outs):  # same release adds and drops a name (a symbol swap): no change
                del ins[c], outs[c]
            for c, e in ins.items():
                if c not in cur:
                    errors.append(f"{idx} {when}: {e['release']} adds {e['symbol']} ({c}) but it is not a member after that date")
                    continue
                spells.append((c, when, cur.pop(c), e["release"], removed_by.pop(c)))
            for c, e in outs.items():
                if c in cur:
                    errors.append(f"{idx} {when}: {e['release']} drops {e['symbol']} ({c}) but it is still a member after that date")
                    continue
                cur[c] = when
                removed_by[c] = e["release"]
            want = expected_size(ov, idx, size, when - dt.timedelta(days=1))
            if len(cur) != want:
                errors.append(f"{idx} {when}: {len(cur)} members before this change, want {want} "
                              f"({', '.join(sorted(set(e['release'] for e in group)))})")
            all_rows += [dict(e, when=when) for e in group]
        for c, end in cur.items():
            spells.append((c, d(first), end, "start", removed_by[c]))
        spells.sort(key=lambda s: (s[0], s[1]))
        out = f"internal/data/{idx}_members.csv"
        with open(out, "w", newline="") as f:
            w = csv.writer(f, lineterminator="\n")
            w.writerow(["symbol", "from", "to", "added_by", "removed_by"])
            for c, a, b, ab, rb in spells:
                w.writerow([c, a.isoformat(), b.isoformat() if b else "", ab, rb])
        print(f"members: {idx}: {len(spells)} spells, {len({s[0] for s in spells})} symbols, {first} to {fetched} -> {out}")
    with open(CHANGES_OUT, "w", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        w.writerow(["index", "effective", "action", "symbol", "release_symbol", "company", "release"])
        for e in sorted(all_rows, key=lambda e: (e["index"], e["date"], e["action"], e["canon"])):
            w.writerow([e["index"], e["date"].isoformat(), e["action"], e["canon"], e["symbol"], e["name"], e["release"]])
    if errors:
        sys.exit("members: the walk back does not reconcile:\n  " + "\n  ".join(errors))
    check_anchors(syms)


def check_anchors(syms):
    """Compare the rebuilt lists with every past copy of NSE's own constituent CSV the Internet Archive saved
    (nse_fetch.py anchors). Each must match the rebuilt list on the day it was captured, symbol for symbol."""
    root = os.path.join(RAW, "anchors")
    files = sorted(os.listdir(root)) if os.path.isdir(root) else []
    if not files:
        sys.exit("members: no archived constituent lists to check against; run research/py/nse_fetch.py anchors")
    spells = {}
    for idx in INDICES:
        with open(f"internal/data/{idx}_members.csv", newline="") as f:
            spells[idx] = [(r["symbol"], d(r["from"]), d(r["to"]) if r["to"] else dt.date.max) for r in csv.DictReader(f)]
    bad, checked = [], defaultdict(list)
    for name in files:
        idx = "nifty50" if name.startswith("ind_nifty50list_") else "niftymidcap150"
        day = dt.datetime.strptime(name.rsplit("_", 1)[1][:8], "%Y%m%d").date()
        raw = open(os.path.join(root, name), "rb").read()
        if raw[:2] == b"\x1f\x8b":
            raw = gzip.decompress(raw)  # some captures were stored with their transfer compression
        rows = list(csv.DictReader(io.StringIO(raw.decode("utf-8-sig", "replace"))))
        col = next((k for k in (rows[0] if rows else {}) if k and k.strip().lower() == "symbol"), None)
        if col is None:
            print(f"members: anchor {name} is not a constituent list; skipped")
            continue
        theirs = {syms.canonical(r[col].strip(), day) for r in rows if r[col].strip()}
        ours = {s for s, a, b in spells[idx] if a <= day < b}
        checked[idx].append(day.isoformat())
        if theirs != ours:
            bad.append(f"{name}: only in NSE's list {sorted(theirs - ours)}, only in ours {sorted(ours - theirs)}")
    for idx, days in sorted(checked.items()):
        print(f"members: {idx}: matches all {len(days)} archived NSE lists ({days[0]} to {days[-1]})" if not bad else
              f"members: {idx}: {len(days)} archived NSE lists checked")
    if bad:
        sys.exit("members: rebuilt lists disagree with archived NSE lists:\n  " + "\n  ".join(bad))


# ---------------------------------------------------------------- prices

PRICE_DIR = os.path.join(RAW, "prices")
# Prices start a year before the first member list (Nifty 50, 2008-01-01), enough for a 12-month look-back. NSE's
# corporate-action records are patchy before 2007, so earlier bars would carry unadjusted splits.
PRICE_START = dt.date(2007, 1, 1)
SERIES_OK = ("EQ", "BE", "BZ", "RR")  # BE/BZ: moved to trade-for-trade (same equity); RR: REIT units (in indices since 2026)


def universe_symbols():
    out = set()
    for idx in INDICES:
        with open(f"internal/data/{idx}_members.csv", newline="") as f:
            out |= {r["symbol"] for r in csv.DictReader(f)}
    return sorted(out)


def num(x):
    try:
        return float(x)
    except (TypeError, ValueError):
        return 0.0


def read_bhav(path):
    """Yield (symbol, series, open, high, low, close, volume) from one bhavcopy zip in either format."""
    with zipfile.ZipFile(path) as z:
        text = z.read(z.namelist()[0]).decode("latin-1")
    rows = csv.reader(io.StringIO(text))
    head = [h.strip() for h in next(rows)]
    if head[0] == "SYMBOL":
        for r in rows:
            if len(r) >= 9:
                yield r[0].strip(), r[1].strip(), num(r[2]), num(r[3]), num(r[4]), num(r[5]), int(num(r[8]))
    else:
        c = {h: i for i, h in enumerate(head)}
        for r in rows:
            if len(r) > c["TtlTradgVol"] and r[c["Sgmt"]] == "CM":
                yield (r[c["TckrSymb"]].strip(), r[c["SctySrs"]].strip(), num(r[c["OpnPric"]]), num(r[c["HghPric"]]),
                       num(r[c["LwPric"]]), num(r[c["ClsPric"]]), int(num(r[c["TtlTradgVol"]])))


def raw_series(syms, canon_list):
    """canonical symbol -> {date: (open, high, low, close, volume, traded symbol, series)} from every bhavcopy."""
    spans = defaultdict(list)  # traded symbol -> [(from, to, canonical)]
    for c in canon_list:
        for sym, a, b in syms.history(c):
            spans[sym].append((a or dt.date.min, b or dt.date.max, c))
    out = {c: {} for c in canon_list}
    files = sorted(os.path.join(dp, f) for dp, _, fs in os.walk(os.path.join(RAW, "bhav")) for f in fs if f.endswith(".zip"))
    if len(files) < 5000:
        sys.exit(f"prices: only {len(files)} bhavcopy files; run research/py/nse_fetch.py bhav")
    for n, path in enumerate(files):
        day = dt.datetime.strptime(os.path.basename(path)[:8], "%Y%m%d").date()
        if day < PRICE_START:
            continue
        for sym, ser, o, h, lo, cl, v in read_bhav(path):
            if sym not in spans or ser not in SERIES_OK or cl <= 0:
                continue
            for a, b, c in spans[sym]:
                if a <= day < b:
                    prev = out[c].get(day)
                    if prev is None or (prev[6] != "EQ" and ser == "EQ"):
                        out[c][day] = (o or cl, h or cl, lo or cl, cl, v, sym, ser)
        if n % 1000 == 0:
            print(f"prices: read {n}/{len(files)} bhavcopies", flush=True)
    return out


AMOUNT = r"(?:rs|re|inr)?\.?\s*-?\s*([0-9]+(?:\.[0-9]+)?)"


def classify(subject, face):
    """Parse one NSE corporate-action subject into [(kind, details)] - kinds: dividend, bonus, split, rights, demerger."""
    s = " ".join(subject.lower().replace("/-", " ").split())
    out = []
    # "Bonus Debentures 6:1" or "Bonus Ncrps 4:1" hand out debt or preference shares, not equity: their value leaves the
    # share price like a demerger's does, so they are measured from the ex-date price below, never as a share bonus.
    bonus_security = re.search(r"bonus\s*(?:deb|ncrps|nccps|ncd|preference|pref|redeemable)", s)
    m = re.search(r"(?:bonus|\bbon\b)\D{0,30}?(\d+)\s*:\s*(\d+)", s)
    if m and not bonus_security:
        out.append(("bonus", {"new": int(m.group(1)), "held": int(m.group(2))}))
    m = re.search(r"(?:split|sub-?division|\bspl\b|\bfv\b)\D*?" + AMOUNT + r"\D*?to\s*" + AMOUNT, s)
    if m and "consolidation" not in s and float(m.group(1)) != float(m.group(2)):
        out.append(("split", {"from": float(m.group(1)), "to": float(m.group(2))}))
    m = re.search(r"consolidation\D*?" + AMOUNT + r"\D*?to\s*" + AMOUNT, s)
    if m and "reduction" not in s:
        out.append(("split", {"from": float(m.group(1)), "to": float(m.group(2))}))
    if re.search(r"\brights?\b", s):
        # "Rights 3:25 @ Premium Rs 1790", "Rights 87:38 @ Premium Of Rs 2.50", "Rights 1:10 @ Prem Rs 197", or several
        # tranches in one record ("4:25 Fully Paid ... @ Premium Rs 500 / 2:25 Partly Paid ... @ Premium Rs 605").
        # A partly paid share is priced at its full issue price; the calls are part of the price paid.
        tranches = []
        for m in re.finditer(r"(\d+)\s*:\s*(\d+)\b(.*?)(?=\d+\s*:\s*\d+\b|$)", s):
            pm = re.search(r"(?:rs|re|inr)\.?\s*([0-9]+(?:\.[0-9]+)?)", m.group(3))
            if pm:
                premium = "prem" in m.group(3)[:pm.start()]
                tranches.append((int(m.group(1)), int(m.group(2)), float(pm.group(1)) + (face if premium else 0.0)))
        if tranches and len({t[1] for t in tranches}) == 1:
            out.append(("rights", {"new": [t[0] for t in tranches], "held": tranches[0][1], "price": [t[2] for t in tranches]}))
        elif tranches:
            out.append(("rights?", {"subject": s}))
    if re.search(r"demerger|arrangement|arngmnt|arrgmt|\bagmt\b|capital reduction|reduction of capital|spin", s):
        # "Demerger" always hands shareholders a new company; a bare "Scheme of Arrangement" often does not (mergers,
        # bonus debentures, internal transfers), so only a visible ex-date drop counts for it.
        out.append(("demerger", {"explicit": True} if re.search(r"demerg|spin", s) or bonus_security else {}))
    elif bonus_security:
        out.append(("demerger", {"explicit": True}))
    # Dividends: every amount in the parts (split at "/", "and", "+", "&") that speak of a dividend, so "Final Rs 22 +
    # Special Rs 10" is 32 and "Bonus 1:2 And Dividend Rs.8/-" is 8.
    # A percentage is of the face value; when the same record splits the shares, of the face value before the split.
    split = next((x for k, x in out if k == "split"), None)
    face_before = split["from"] if split and abs(split["to"] - face) < 1e-9 else face
    amt = 0.0
    for part in re.split(r"/|\band\b|\+|&", s):
        if not re.search(r"\bdiv(?!ision)|special|final|interim", part) or \
                re.search(r"split|\bspl\b|bonus|\bbon\b|rights|\bfv\b|face value|consolidat", part):
            continue
        for m in re.finditer(r"(rs|re|inr)?\.?\s*-?\s*([0-9]+(?:\.[0-9]+)?)\s*(%)?", part):
            if m.group(3) == "%":
                amt += float(m.group(2)) * face_before / 100
            elif m.group(1):
                amt += float(m.group(2))
    if amt > 0:
        out.append(("dividend", {"amount": round(amt, 4)}))
    return out


def build_prices():
    ov = json.load(open(OVERRIDES))
    ov.setdefault("symbol_changes", [])
    cov = json.load(open(CA_OVERRIDES))
    syms = Symbols(ov)
    canon = universe_symbols()
    series = raw_series(syms, canon)
    empty = [c for c in canon if not series[c]]
    if empty:
        sys.exit(f"prices: no bhavcopy bars at all for {', '.join(empty)}; add the missing rename to {OVERRIDES}")

    # Corporate actions for the universe. NSE files them under the company's current symbol; map through the renames.
    recs = []
    for f in sorted(os.listdir(os.path.join(RAW, "ca"))):
        if f.endswith(".json"):
            recs += json.load(open(os.path.join(RAW, "ca", f)))
    events = {}
    for r in recs:
        if r.get("series") not in (None, "", "EQ", "BE", "BZ", "-"):
            continue
        ex = dt.datetime.strptime(r["exDate"], "%d-%b-%Y").date()
        if ex <= PRICE_START:
            continue
        c = syms.canonical(r["symbol"].strip(), ex)
        if c not in series:
            continue
        subject = " ".join(r["subject"].split())
        for kind, det in classify(r["subject"], num(r.get("faceVal"))):
            key = (c, ex, kind)
            if key in events and kind == "dividend":
                # Several dividend records on one day: different amounts are separate payouts (interim plus final,
                # regular plus special) and add up; the same amount is the same payout filed twice, unless one record is
                # an interim and the other is not.
                seen = events[key]["det"]["parts"]
                part = (det["amount"], "interim" in subject.lower())
                if part not in seen:
                    seen.append(part)
                    events[key]["det"]["amount"] = round(sum(a for a, _ in seen), 4)
                    events[key]["subject"] += " | " + subject
                continue
            if kind == "dividend":
                det = dict(det, parts=[(det["amount"], "interim" in subject.lower())])
            events[key] = {"symbol": c, "ex": ex, "kind": kind, "det": det, "subject": subject}
    for e in cov.get("drop", []):
        events.pop((e["symbol"], d(e["ex"]), e["kind"]), None)
    for e in cov.get("add", []):
        events[(e["symbol"], d(e["ex"]), e["kind"])] = {"symbol": e["symbol"], "ex": d(e["ex"]), "kind": e["kind"],
                                                       "det": e.get("det", {}), "subject": e["why"]}

    # Price factor for each event: every price strictly before the ex-date is multiplied by it.
    rows, problems = [], []
    for (c, ex, kind), e in sorted(events.items()):
        bars = series[c]
        days = sorted(bars)
        before = [x for x in days if x < ex]
        on = [x for x in days if x >= ex]
        if kind != "dividend" and (not before or not on or (on[0] - ex).days > 10):
            continue  # outside the bars we hold (before listing or after delisting)
        prev_close = bars[before[-1]][3] if before else 0.0
        factor, method, amount = 1.0, "", 0.0
        det = e["det"]
        if kind == "bonus":
            factor, method = det["held"] / (det["held"] + det["new"]), f"bonus {det['new']}:{det['held']}"
        elif kind == "split":
            factor, method = det["to"] / det["from"], f"face value {det['from']:g} to {det['to']:g}"
        elif kind == "rights":
            new, b, prices = det["new"], det["held"], det["price"]
            terp = (b * prev_close + sum(a * p for a, p in zip(new, prices))) / (b + sum(new))
            factor = min(1.0, terp / prev_close) if prev_close else 1.0  # a rights issue priced above the market is ignored
            method = "rights " + " + ".join(f"{a}:{b} at {p:g}" for a, p in zip(new, prices)) + ", theoretical ex-rights price"
        elif kind == "rights?":
            problems.append(f"{c} {ex}: cannot read the rights terms ({e['subject']}); add the event to {CA_OVERRIDES}")
            continue
        elif kind == "demerger":
            if "factor" in det:
                factor, method = det["factor"], "hand-checked factor"
            else:
                o, cl = bars[on[0]][0], bars[on[0]][3]
                if o > 0 and abs(o / cl - 1) <= 0.10:
                    factor, method = o / prev_close, "ex-date open (special pre-open price) / previous close"
                else:
                    factor, method = cl / prev_close, "ex-date close / previous close"
                if factor > 0.97 and not det.get("explicit"):
                    factor, method = 1.0, "no price effect (ex-date move under 3%)"
                elif factor > 1.0:
                    factor, method = 1.0, "no price effect (opened above the previous close)"
        elif kind == "dividend":
            amount, method = det["amount"], "cash dividend per share"
        if not (0.0 < factor <= 1.0 or kind == "split"):
            problems.append(f"{c} {ex} {kind}: factor {factor:.4f} is out of range ({e['subject']})")
        rows.append({"symbol": c, "ex_date": ex, "kind": kind, "factor": factor, "dividend": amount, "method": method,
                     "nse_subject": e["subject"]})

    # Back-adjust and write one file per symbol in the layout internal/panel reads.
    os.makedirs(PRICE_DIR, exist_ok=True)
    by_sym = defaultdict(list)
    for r in rows:
        by_sym[r["symbol"]].append(r)
    for c in canon:
        bars = series[c]
        days = sorted(bars)
        adj = [r for r in by_sym[c] if r["kind"] != "dividend" and r["factor"] != 1.0]
        def cum(day):
            f = 1.0
            for r in adj:
                if day < r["ex_date"]:
                    f *= r["factor"]
            return f
        # A dividend going ex with a split or bonus is paid on the shares held before it (TCS, 31 May 2018: Rs 29 and a
        # 1:1 bonus), so it is scaled by that day's factor too.
        divs = {r["ex_date"]: r["dividend"] * cum(r["ex_date"] - dt.timedelta(days=1)) for r in by_sym[c] if r["kind"] == "dividend"}
        tr, tr_f = {}, 1.0  # total-return factor for AdjClose, rolled back from the last bar
        for i in range(len(days) - 1, -1, -1):
            tr[days[i]] = tr_f
            dv = divs.get(days[i], 0.0)
            if dv and i > 0:
                pc = bars[days[i - 1]][3] * cum(days[i - 1])
                if pc > dv:
                    tr_f *= 1 - dv / pc
        with open(os.path.join(PRICE_DIR, c.replace("&", "_and_") + ".csv"), "w", newline="") as f:
            w = csv.writer(f, lineterminator="\n")
            w.writerow(["Date", "Open", "High", "Low", "Close", "AdjClose", "Volume"])
            for day in days:
                o, h, lo, cl, v, _, _ = bars[day]
                k = cum(day)
                w.writerow([day.isoformat(), *(f"{x * k:.4f}" for x in (o, h, lo, cl)), f"{cl * k * tr[day]:.4f}", v])
        with open(os.path.join(PRICE_DIR, c.replace("&", "_and_") + "_div.csv"), "w", newline="") as f:
            w = csv.writer(f, lineterminator="\n")
            w.writerow(["Date", "Dividend"])
            for day in sorted(divs):
                if day >= days[0]:
                    w.writerow([day.isoformat(), f"{divs[day]:.4f}"])
    with open(CA_OUT, "w", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        w.writerow(["symbol", "ex_date", "kind", "price_factor", "dividend", "method", "nse_subject"])
        for r in sorted(rows, key=lambda r: (r["symbol"], r["ex_date"], r["kind"])):
            w.writerow([r["symbol"], r["ex_date"].isoformat(), r["kind"], f"{r['factor']:.6f}",
                        f"{r['dividend']:g}" if r["kind"] == "dividend" else "", r["method"], r["nse_subject"]])
    n_adj = sum(1 for r in rows if r["kind"] != "dividend" and r["factor"] != 1.0)
    print(f"prices: {len(canon)} symbols, {n_adj} price adjustments, "
          f"{sum(1 for r in rows if r['kind'] == 'dividend')} dividends -> {PRICE_DIR}, {CA_OUT}")
    if problems:
        sys.exit("prices: corporate-action problems:\n  " + "\n  ".join(problems))
    audit_prices(canon, cov)


AUDIT_OUT = "research/nse/price_audit.csv"
YAHOO_DIR = os.environ.get("YAHOO_DIR", ".research-data/yahoo")


def yahoo_returns(sym):
    """Yahoo's close-to-close returns for one symbol, if research/py/fetch_yahoo.py has downloaded it (optional)."""
    aliases = json.load(open("research/symbol_aliases.json"))
    for ticker in (aliases.get(sym), sym + ".NS"):
        if not ticker:
            continue
        path = os.path.join(YAHOO_DIR, ticker.replace("&", "_and_") + ".csv")
        if os.path.exists(path):
            rows = [r for r in csv.DictReader(open(path)) if num(r["Close"]) > 0]
            return {d(b["Date"]): num(b["Close"]) / num(a["Close"]) - 1 for a, b in zip(rows, rows[1:])}
    return None


def audit_prices(canon, cov):
    """List every suspicious daily move left in the adjusted series - more than 30% close to close, or more than 15%
    that opened that way and held - over each symbol's whole history (a wrong factor outside the membership years still
    signals a parsing error). A move passes if it happened during the session (opened within 10% of the
    previous close), if Yahoo's independent history shows the same move that day (within 3 points), or if
    research/nse/corporate_actions_overrides.json records it as real with a reason. Anything else is almost always a
    corporate action NSE's records miss: it fails the build when it falls where a run can see it (from 400 days before
    the symbol's first membership, the longest look-back here, to its last) and is listed as "outside use" otherwise."""
    spans = defaultdict(list)
    for idx in INDICES:
        with open(f"internal/data/{idx}_members.csv", newline="") as f:
            for r in csv.DictReader(f):
                spans[r["symbol"]].append((d(r["from"]), d(r["to"]) if r["to"] else dt.date.max))
    real = {(m["symbol"], m["date"]): m["why"] for m in cov.get("real_moves", [])}
    rows, bad = [], []
    for c in canon:
        with open(os.path.join(PRICE_DIR, c.replace("&", "_and_") + ".csv"), newline="") as f:
            bars = [(d(r["Date"]), num(r["Open"]), num(r["Close"])) for r in csv.DictReader(f)]
        yret = None
        for (_, _, pc), (day, o, cl) in zip(bars, bars[1:]):
            ret, gap = cl / pc - 1, o / pc - 1
            if not (abs(ret) > 0.30 or (ret < -0.15 and gap < -0.15) or (ret > 0.15 and gap > 0.15)):
                continue
            if yret is None:
                yret = yahoo_returns(c) or {}
            y = yret.get(day)
            if abs(gap) <= 0.10 and abs(ret) > 0.30:
                status = "intraday move"
            elif y is not None and abs(y - ret) <= 0.03:
                status = "confirmed by Yahoo"
            elif (c, day.isoformat()) in real:
                status = "real: " + real[(c, day.isoformat())]
            elif not any(a - dt.timedelta(days=400) <= day < b for a, b in spans[c]):
                status = "unexplained, outside use"
            else:
                status = "UNEXPLAINED"
                bad.append(f"{c} {day} {ret * 100:+.1f}% (open {gap * 100:+.1f}%, Yahoo "
                           f"{'none' if y is None else f'{y * 100:+.1f}%'})")
            member = any(a <= day < b for a, b in spans[c])
            rows.append([c, day.isoformat(), f"{ret * 100:.1f}", f"{gap * 100:.1f}",
                         "" if y is None else f"{y * 100:.1f}", "yes" if member else "no", status])
    with open(AUDIT_OUT, "w", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        w.writerow(["symbol", "date", "close_to_close_pct", "open_gap_pct", "yahoo_pct", "index_member", "status"])
        w.writerows(rows)
    print(f"prices: audit: {len(rows)} large moves, {len(bad)} unexplained -> {AUDIT_OUT}")
    if bad:
        sys.exit("prices: unexplained moves (a missed corporate action, or a real move to record in "
                 f"{CA_OVERRIDES} real_moves):\n  " + "\n  ".join(bad))


def main():
    steps = sys.argv[1:] or ["members", "prices"]
    for s in steps:
        {"members": build_members, "prices": build_prices}[s]()


if __name__ == "__main__":
    main()
