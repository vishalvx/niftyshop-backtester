#!/usr/bin/env python3
"""Download daily OHLCV + dividends + splits from Yahoo's chart API into .research-data/yahoo/.
Usage: fetch_yahoo.py SYMBOL [SYMBOL...]   (SYMBOL is the Yahoo ticker, e.g. TCS.NS or ^NSEI)
Writes <out>/<safe>.csv, <safe>_div.csv, <safe>_split.csv; records failures in <out>/_failures.tsv.
"""
import sys, os, json, time, subprocess, urllib.parse, datetime as dt

OUT = os.environ.get("YAHOO_OUT", ".research-data/yahoo")
UA = {"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/124.0 Safari/537.36"}
P1 = int(dt.datetime(2006, 6, 1).timestamp())
P2 = int(dt.datetime(2026, 10, 2).timestamp())

def safe(sym): return sym.replace("^", "_idx_").replace("&", "_and_").replace("/", "_")

class Resp:
    def __init__(self, code, body): self.status_code, self.text = code, body
    def json(self): return json.loads(self.text)

def fetch(sym, retries=6):
    # Yahoo rate-limits python-requests' TLS fingerprint (429) but serves curl, so shell out to curl.
    q = urllib.parse.urlencode(dict(period1=P1, period2=P2, interval="1d", events="div|split", includeAdjustedClose="true"))
    url = f"https://query2.finance.yahoo.com/v8/finance/chart/{urllib.parse.quote(sym)}?{q}"
    for a in range(retries):
        out = subprocess.run(["curl", "-s", "-m", "40", "-A", "Mozilla/5.0", "-w", "\n%{http_code}", url],
                             capture_output=True, text=True).stdout
        body, _, code = out.rpartition("\n")
        if code == "429" or body.startswith("Too Many"):
            time.sleep(8 * (a + 1)); continue
        return Resp(int(code or 0), body)
    return Resp(429, "")

def main():
    os.makedirs(OUT, exist_ok=True)
    fails = open(os.path.join(OUT, "_failures.tsv"), "a")
    for sym in sys.argv[1:]:
        s = safe(sym)
        if os.path.exists(os.path.join(OUT, s + ".csv")):
            continue
        try:
            r = fetch(sym)
            js = r.json()
            res = js.get("chart", {}).get("result")
            if not res:
                err = js.get("chart", {}).get("error")
                fails.write(f"{sym}\t{r.status_code}\t{json.dumps(err)}\n"); fails.flush()
                print("FAIL", sym, err); continue
            res = res[0]
            off = res["meta"].get("gmtoffset", 19800)
            ts = res.get("timestamp") or []
            q = res["indicators"]["quote"][0]
            adj = (res["indicators"].get("adjclose") or [{}])[0].get("adjclose") or [None] * len(ts)
            def d(t): return dt.datetime.fromtimestamp(t + off, dt.timezone.utc).strftime("%Y-%m-%d")
            with open(os.path.join(OUT, s + ".csv"), "w") as f:
                f.write("Date,Open,High,Low,Close,AdjClose,Volume\n")
                for i, t in enumerate(ts):
                    c = q["close"][i]
                    if c is None: continue
                    f.write(",".join([d(t), *(("" if q[k][i] is None else repr(q[k][i])) for k in ("open", "high", "low", "close")),
                                      "" if adj[i] is None else repr(adj[i]), str(q["volume"][i] or 0)]) + "\n")
            ev = res.get("events") or {}
            with open(os.path.join(OUT, s + "_div.csv"), "w") as f:
                f.write("Date,Dividend\n")
                for k, v in sorted((ev.get("dividends") or {}).items(), key=lambda kv: int(kv[0])):
                    f.write(f"{d(v['date'])},{v['amount']}\n")
            with open(os.path.join(OUT, s + "_split.csv"), "w") as f:
                f.write("Date,Numerator,Denominator\n")
                for k, v in sorted((ev.get("splits") or {}).items(), key=lambda kv: int(kv[0])):
                    f.write(f"{d(v['date'])},{v['numerator']},{v['denominator']}\n")
            print("OK", sym, len(ts), d(ts[0]) if ts else "-", d(ts[-1]) if ts else "-")
        except Exception as e:
            fails.write(f"{sym}\texc\t{e!r}\n"); fails.flush(); print("EXC", sym, e)
        time.sleep(0.35)

if __name__ == "__main__":
    main()
