#!/usr/bin/env python3
"""Download the official NSE raw data behind the point-in-time member lists and prices into .research-data/nse/.

Usage: nse_fetch.py [lists] [press] [anchors] [ca] [bhav]   (no argument = all five)

  bhav   every NSE cash-market bhavcopy (daily official OHLC) from 2006-01-01 to yesterday:
         old format  nsearchives.nseindia.com/content/historical/EQUITIES/YYYY/MON/cmDDMONYYYYbhav.csv.zip (to 2024-07-05)
         UDiFF       nsearchives.nseindia.com/content/cm/BhavCopy_NSE_CM_0_0_0_YYYYMMDD_F_0000.csv.zip (from 2024-07-08)
         Every calendar day is tried (NSE has held Saturday and Sunday sessions); a 404 is a non-trading day.
  ca     NSE corporate actions for equities, one query per calendar month from 2005 (www.nseindia.com/api/corporates-corporateActions).
  press  every NSE Indices press release from 2007 that may change an equity index (niftyindices.com/Press_Release/*.pdf),
         converted to text with poppler's `pdftotext -layout`.
  lists  today's Nifty 50 and Nifty Midcap 150 constituent CSVs, NSE's symbol-change file and equity list.
  anchors  past copies of those constituent CSVs saved by the Internet Archive, used only to check the rebuilt lists.

Re-running is safe: files already on disk are skipped, and so are days already recorded as non-trading in bhav/_closed.txt.
Raw data is large (about 400 MB, mostly bhavcopy zips) and stays out of git; research/py/nse_build.py turns it into the committed lists.
"""
import datetime as dt
import html
import json
import os
import re
import subprocess
import sys
import time
import urllib.parse
from concurrent.futures import ThreadPoolExecutor

OUT = os.environ.get("NSE_OUT", ".research-data/nse")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36"
FIRST_DAY = dt.date(2006, 1, 1)
UDIFF_FROM = dt.date(2024, 7, 8)


def curl(url, dest=None, extra=()):
    """GET url; returns (http status, body bytes or None). Writes to dest atomically when given and status is 200."""
    for attempt in range(5):
        tmp = (dest + ".part") if dest else None
        args = ["curl", "-s", "-L", "-m", "90", "-A", UA, "-w", "%{http_code}", *extra, url]
        args += ["-o", tmp] if dest else ["-o", "-"]
        p = subprocess.run(args, capture_output=True)
        if dest:
            code = int(p.stdout.decode() or 0)
            body = None
        else:
            body, code = p.stdout[:-3], int(p.stdout[-3:].decode() or 0)
        if code == 200:
            if dest:
                os.replace(tmp, dest)
            return code, body
        if tmp and os.path.exists(tmp):
            os.remove(tmp)
        if code == 404:
            return code, None
        time.sleep(5 * (attempt + 1))  # 403/429/5xx/timeouts: back off and retry
    return code, None


def bhav_url(d):
    if d >= UDIFF_FROM:
        return f"https://nsearchives.nseindia.com/content/cm/BhavCopy_NSE_CM_0_0_0_{d:%Y%m%d}_F_0000.csv.zip"
    mon = d.strftime("%b").upper()
    return f"https://nsearchives.nseindia.com/content/historical/EQUITIES/{d.year}/{mon}/cm{d:%d}{mon}{d.year}bhav.csv.zip"


def fetch_bhav():
    root = os.path.join(OUT, "bhav")
    os.makedirs(root, exist_ok=True)
    closed_path = os.path.join(root, "_closed.txt")
    closed = set(open(closed_path).read().split()) if os.path.exists(closed_path) else set()
    today = dt.date.today()
    days, d = [], FIRST_DAY
    while d < today:
        if d.isoformat() not in closed and not os.path.exists(os.path.join(root, str(d.year), f"{d:%Y%m%d}.csv.zip")):
            days.append(d)
        d += dt.timedelta(days=1)
    print(f"bhav: {len(days)} days to try", flush=True)
    failed = []

    def one(d):
        os.makedirs(os.path.join(root, str(d.year)), exist_ok=True)
        code, _ = curl(bhav_url(d), os.path.join(root, str(d.year), f"{d:%Y%m%d}.csv.zip"))
        return d, code

    with ThreadPoolExecutor(4) as ex, open(closed_path, "a") as cf:
        for i, (d, code) in enumerate(ex.map(one, days)):
            if code == 404:
                # Archive files can appear a day late; only record a day as closed once it is a week old.
                if (today - d).days > 7:
                    cf.write(d.isoformat() + "\n")
            elif code != 200:
                failed.append((d, code))
            if i % 250 == 0:
                cf.flush()
                print(f"bhav: {i}/{len(days)} {d} {code}", flush=True)
    for d, code in failed:
        print(f"bhav: FAILED {d} HTTP {code}", file=sys.stderr)
    if failed:
        sys.exit(f"bhav: {len(failed)} days failed to download; re-run to retry")


def fetch_ca():
    root = os.path.join(OUT, "ca")
    os.makedirs(root, exist_ok=True)
    jar = os.path.join(root, ".cookies")
    curl("https://www.nseindia.com/companies-listing/corporate-filings-actions", extra=("-c", jar))
    today = dt.date.today()
    y, m = 2005, 1
    while (y, m) <= (today.year, today.month):
        first = dt.date(y, m, 1)
        last = (dt.date(y + (m == 12), m % 12 + 1, 1) - dt.timedelta(days=1))
        path = os.path.join(root, f"{y}-{m:02d}.json")
        current = last >= today - dt.timedelta(days=7)  # refresh the latest months on every run
        if current or not os.path.exists(path):
            url = ("https://www.nseindia.com/api/corporates-corporateActions?index=equities"
                   f"&from_date={first:%d-%m-%Y}&to_date={last:%d-%m-%Y}")
            code, body = curl(url, extra=("-b", jar, "-c", jar, "-H", "Accept: application/json",
                                          "-H", "Referer: https://www.nseindia.com/companies-listing/corporate-filings-actions"))
            if code != 200:
                sys.exit(f"ca: HTTP {code} for {url}")
            rows = json.loads(body)
            if not isinstance(rows, list):
                sys.exit(f"ca: unexpected response for {url}: {body[:200]!r}")
            with open(path + ".part", "w") as f:
                json.dump(rows, f, indent=0)
            os.replace(path + ".part", path)
            print(f"ca: {y}-{m:02d} {len(rows)} records", flush=True)
            time.sleep(1)
        y, m = (y + (m == 12), m % 12 + 1)


# Press releases that cannot change Nifty 50 or Nifty Midcap 150 membership, by title.
SKIP_TITLE = re.compile(r"fixed income|\bsdl\b|g-?sec|\bbond|\bsme\b|\bipo\b|t-?bill|gilt|money market", re.I)


def fetch_press():
    root = os.path.join(OUT, "press")
    os.makedirs(root, exist_ok=True)
    code, body = curl("https://www.niftyindices.com/press-release")
    if code != 200:
        sys.exit(f"press: HTTP {code} for the press-release index")
    page = body.decode("utf-8", "replace")
    items = re.findall(r'data-date="([^"]+)"[^>]*>\s*<p>[^<]*</p>\s*<a href=\'([^\']+\.pdf)\'[^>]*>([^<]*)</a>', page)
    if len(items) < 1000:
        sys.exit(f"press: only {len(items)} releases listed; the page layout may have changed")
    index = []
    for date, href, title in items:
        d = dt.datetime.strptime(date, "%b %d, %Y").date()
        title = " ".join(html.unescape(title).split())
        index.append({"date": d.isoformat(), "file": os.path.basename(href), "title": title,
                      "wanted": d.year >= 2007 and not SKIP_TITLE.search(title)})
    with open(os.path.join(root, "_index.json"), "w") as f:
        json.dump(index, f, indent=1)
    wanted = list({r["file"]: r for r in index if r["wanted"]}.values())  # a few releases are listed twice
    print(f"press: {len(items)} releases listed, {len(wanted)} to fetch", flush=True)

    def one(r):
        pdf = os.path.join(root, r["file"])
        gone = pdf + ".notpdf"
        if os.path.exists(gone):
            return r["file"], "not a PDF"
        if not os.path.exists(pdf):
            code, _ = curl("https://www.niftyindices.com/Press_Release/" + r["file"], pdf)
            if code != 200:
                return r["file"], f"HTTP {code}"
            with open(pdf, "rb") as f:
                if f.read(5) != b"%PDF-":
                    # A few listed links serve the site's HTML page instead of the release; keep a marker so the
                    # build can report them, and do not retry on every run.
                    os.replace(pdf, gone)
                    return r["file"], "not a PDF"
        txt = pdf[:-4] + ".txt"
        if not os.path.exists(txt):
            subprocess.run(["pdftotext", "-layout", pdf, txt], check=True, stderr=subprocess.DEVNULL)
        return r["file"], "ok"

    with ThreadPoolExecutor(4) as ex:
        res = list(ex.map(one, wanted))
    for f, c in res:
        if c != "ok":
            print(f"press: {f}: {c}", file=sys.stderr)
    bad = [f for f, c in res if c.startswith("HTTP")]
    if bad:
        sys.exit(f"press: {len(bad)} releases failed to download; re-run to retry")


LISTS = {
    "ind_nifty50list.csv": "https://nsearchives.nseindia.com/content/indices/ind_nifty50list.csv",
    "ind_niftymidcap150list.csv": "https://nsearchives.nseindia.com/content/indices/ind_niftymidcap150list.csv",
    "symbolchange.csv": "https://nsearchives.nseindia.com/content/equities/symbolchange.csv",
    "EQUITY_L.csv": "https://nsearchives.nseindia.com/content/equities/EQUITY_L.csv",
}


def fetch_lists():
    root = os.path.join(OUT, "lists")
    os.makedirs(root, exist_ok=True)
    stamp = dt.date.today().isoformat()
    for name, url in LISTS.items():
        code, _ = curl(url, os.path.join(root, name))
        if code != 200:
            sys.exit(f"lists: HTTP {code} for {url}")
    with open(os.path.join(root, "_fetched.txt"), "w") as f:
        f.write(stamp + "\n")
    print(f"lists: {len(LISTS)} files as of {stamp}")


ANCHOR_HOSTS = ["archives.nseindia.com/content/indices/", "nsearchives.nseindia.com/content/indices/",
                "www.nseindia.com/content/indices/", "www1.nseindia.com/content/indices/", "nseindia.com/content/indices/",
                "www.niftyindices.com/IndexConstituent/", "niftyindices.com/IndexConstituent/"]


def fetch_anchors():
    """Past copies of NSE's own constituent CSVs saved by the Internet Archive. They are not an input: nse_build.py checks
    the reconstructed lists against them, so a change missed in the press releases shows up as a mismatch."""
    root = os.path.join(OUT, "anchors")
    os.makedirs(root, exist_ok=True)
    for name in ("ind_nifty50list.csv", "ind_niftymidcap150list.csv"):
        for host in ANCHOR_HOSTS:
            url = host + name
            for attempt in range(6):
                code, body = curl("http://web.archive.org/cdx/search/cdx?" + urllib.parse.urlencode(
                    {"url": url, "output": "json", "filter": "statuscode:200", "collapse": "digest", "fl": "timestamp"}))
                text = (body or b"").decode("utf-8", "replace")
                if code == 200 and text.lstrip().startswith(("[", "")) and "<html" not in text[:200].lower():
                    break
                time.sleep(15 * (attempt + 1))  # the CDX API rate-limits bursts with an HTML error page
            else:
                sys.exit(f"anchors: Internet Archive index query kept failing for {url}")
            stamps = [r[0] for r in (json.loads(text) if text.strip() else [])[1:]]
            for ts in stamps:
                dest = os.path.join(root, f"{name[:-4]}_{ts}.csv")
                if os.path.exists(dest):
                    continue
                code, _ = curl(f"http://web.archive.org/web/{ts}id_/{url}", dest)
                if code != 200:
                    print(f"anchors: HTTP {code} for {ts} {url}", file=sys.stderr)
                time.sleep(2)
            print(f"anchors: {url}: {len(stamps)} snapshots", flush=True)
            time.sleep(5)


def main():
    steps = sys.argv[1:] or ["lists", "press", "anchors", "ca", "bhav"]
    for s in steps:
        {"bhav": fetch_bhav, "ca": fetch_ca, "press": fetch_press, "lists": fetch_lists, "anchors": fetch_anchors}[s]()


if __name__ == "__main__":
    main()
