#!/usr/bin/env python3
"""Download price-index and total-return-index (TRI) history from niftyindices.com into .research-data/indices/.
Usage: fetch_tri.py "NIFTY 50" [index names...]
"""
import sys, os, json, subprocess, tempfile, datetime as dt

OUT = ".research-data/indices"
BASE = "https://www.niftyindices.com"
jar = os.path.join(tempfile.gettempdir(), "niftyindices.jar")

def curl(args):
    return subprocess.run(["curl", "-s", "-m", "60", "-A", "Mozilla/5.0", "-b", jar, "-c", jar, *args], capture_output=True, text=True).stdout

def post(path, name, start, end):
    cinfo = "{'name':'%s','startDate':'%s','endDate':'%s','indexName':'%s'}" % (name, start, end, name)
    body = json.dumps({"cinfo": cinfo})
    out = curl(["-X", "POST", BASE + path, "-H", "Content-Type: application/json; charset=UTF-8",
                "-H", "X-Requested-With: XMLHttpRequest", "-H", "Referer: " + BASE + "/reports/historical-data",
                "--data", body])
    try:
        js = json.loads(out)
        if isinstance(js, dict) and "d" in js:
            js = json.loads(js["d"])
        return js if isinstance(js, list) else None
    except Exception:
        return None

def main():
    os.makedirs(OUT, exist_ok=True)
    curl([BASE + "/reports/historical-data", "-o", "/dev/null"])
    for name in sys.argv[1:]:
        for kind, path in (("TRI", "/BackPage/getTotalReturnIndexString"), ("PRI", "/BackPage/getHistoricaldatatabletoString")):
            rows = []
            for y in range(2005, 2027):
                s, e = f"01-Jan-{y}", f"31-Dec-{y}" if y < 2026 else "01-Oct-2026"
                r = post(path, name, s, e)
                if r: rows += r
            print(name, kind, len(rows), rows[0] if rows else None)
            if rows:
                with open(os.path.join(OUT, f"{name.replace(' ', '_')}_{kind}.json"), "w") as f:
                    json.dump(rows, f)

if __name__ == "__main__":
    main()
