# Official NSE member lists and prices

The `nifty50` and `niftymidcap150` universes run on data rebuilt from NSE's own publications. This folder holds the processed tables and the hand-checked decisions behind them; the raw downloads are not in git.

| File | What it is |
| :--- | :--- |
| [`internal/data/nifty50_members.csv`](../../internal/data/nifty50_members.csv) | Nifty 50 membership spells from 1 Jan 2008 |
| [`internal/data/niftymidcap150_members.csv`](../../internal/data/niftymidcap150_members.csv) | Nifty Midcap 150 membership spells from 30 Sep 2016 |
| [`index_changes.csv`](index_changes.csv) | every change applied: index, effective date, in or out, symbol, the release's own symbol and company name, release |
| [`members_overrides.json`](members_overrides.json) | hand-checked inputs for the lists; each entry says why |
| [`corporate_actions.csv`](corporate_actions.csv) | every price adjustment and dividend used: the factor, how it was computed, and NSE's own wording |
| [`corporate_actions_overrides.json`](corporate_actions_overrides.json) | hand-checked corporate-action inputs, and large daily moves recorded as real trading |
| [`price_audit.csv`](price_audit.csv) | every large daily move left in the adjusted prices and why it is real |

## Rebuild

```bash
caffeinate -i python3 research/py/nse_fetch.py   # raw data into .research-data/nse/ (about 400 MB; 20 to 40 minutes)
python3 research/py/nse_build.py members         # the two member files and index_changes.csv
python3 research/py/nse_build.py prices          # .research-data/nse/prices/ plus corporate_actions.csv and price_audit.csv
```

`nse_fetch.py` needs `curl` and poppler's `pdftotext`; re-running it only fetches what is missing. Both build steps stop with a message naming what to fix if a check fails. Raw data stays in `.research-data/nse/` (gitignored): `bhav/` (one NSE bhavcopy zip per trading day from 2006), `ca/` (NSE corporate-action records by month from 2005), `press/` (NSE Indices press releases from 2007, PDF and text), `lists/` (today's constituent lists and NSE's symbol-change file) and `anchors/` (past copies of NSE's constituent lists from the Internet Archive).

## Member lists

**Sources.** Today's constituent CSV from `nsearchives.nseindia.com/content/indices/`, and every NSE Indices press release that changed the Nifty 50 or the Nifty Midcap 150 (61 releases, 740 changes): semi-annual reviews, ad-hoc replacements, revocations and the voided March 2020 review.

**Method.** `nse_build.py members` starts from today's list and undoes each change, newest first. At every step it checks that each stock a release added is a member after that date, that each stock it dropped is not, and that the count is exactly 50 or 150. It then compares the result with every past copy of NSE's own list the Internet Archive kept. Symbols are canonical: the company's latest NSE symbol, following NSE's symbol-change file (SESAGOA and SSLT are VEDL, ZOMATO is ETERNAL). A merged or delisted company keeps its last symbol (HDFC, SATYAMCOMP).

**Checks passed.**
- Every step of both walks reconciles: 50 (or 150) members on every day.
- All 25 archived NSE lists match symbol for symbol: 19 Nifty 50 lists from Mar 2016 to Jun 2026, and 6 Midcap 150 lists from Feb 2019 to Sep 2026.
- No archived list exists before 2016. For 2008-2016 the Nifty 50 file agrees with the old month-end file (`nifty50_weights.csv`) in all 216 months, apart from naming (BAJAJAUTO for BAJAJ-AUTO), a dated change applied to its whole month (9 months), and the old file leaving out the Tata Motors DVR share (18 months).

**Hand-checked decisions** (`members_overrides.json`):
- Releases from 2007 to 2011 name companies without symbols; `names` maps the 28 names.
- The Nifty 50 held 51 securities from 1 Apr 2016 to 29 Sep 2017, while it included the Tata Motors DVR share (`size_exceptions`). Runs expect 51 on those days.
- The March 2020 review was brought forward for Yes Bank (Nifty 50, 19 Mar 2020) and voided for every other index; the Midcap 150 review was redone for 26 Jun 2020.
- NSE revoked IREDA's Midcap 150 inclusion in March 2024 (BSE went in instead), and in September 2024 the exclusion of Vodafone Idea and the inclusion of Central Bank of India.
- A September 2009 release replaced an earlier one and moved its effective date to 22 Oct 2009.
- Eight releases are scanned images with no text layer. Each was read, and none changes either list. The August 2021 review, which OCR shows makes no Nifty 50 change, had its Midcap 150 list replaced in full in September 2021.
- WABCOINDIA became ZFCVINDIA on 1 Apr 2022, a rename missing from NSE's symbol-change file.

**Not modelled.** After a demerger NSE briefly carries the new company in the index at a dummy price until it lists, for example Jio Financial in 2023 and TMCV in 2025. These entries are left out: they cannot be traded, and the parent stays in.

## Prices

**Source.** NSE's daily bhavcopy, the official end-of-day prices for every listed stock: old format to 5 Jul 2024, UDiFF from 8 Jul 2024. Series EQ, BE and BZ (trade-for-trade) and RR (REIT units). Series start on 2 Jan 2007, a year before the first list, to cover a 12-month look-back. A stock's bars follow it across symbol changes.

**Adjustments.** Every price before an ex-date is multiplied by that event's factor (`corporate_actions.csv`; 499 adjustments):
- **Bonus** a:b (217): b / (a + b).
- **Split or consolidation** (168): new face value / old face value.
- **Rights** a:b at price p (53): the theoretical ex-rights price, (b × P + a × p) / ((a + b) × P), where P is the previous close. Partly paid shares are priced at their full issue price. A rights issue priced above the market is ignored.
- **Demerger** (61) has no published factor; it is estimated from prices. The factor is the ex-date open (NSE's special pre-open session) over the previous close, or the ex-date close if the open is more than 10% from the close. An explicit "Demerger" record is always applied. A bare "Scheme of Arrangement" (often a merger or an internal transfer) is applied only if the ex-date move is more than 3%. Bonus debentures and bonus preference shares are measured the same way.

**Dividends** (7,872) come from the same records and are credited on the ex-date. Several amounts in one record add up ("Final Rs 22 + Special Rs 10"). Separate records on one day add up when the amounts differ; the same amount is one payout filed twice, unless one record is an interim. A dividend that goes ex with a split or bonus is scaled to the new share count. Each dividend is on the same share basis as the adjusted close.

**Checks.**
- `nse_build.py prices` lists every daily move over 30% close to close, or over 15% that opened that way and held. A move passes if the stock opened within 10% of the previous close (the move happened during the session), if Yahoo shows the same move that day within 3 points, or if `real_moves` records it with the evidence. Anything else fails the build if it falls where a run can see it: from 400 days before a symbol's first membership to its last. Result: 211 large moves. 123 are confirmed by Yahoo, 59 happened during the session, and 7 are recorded real events (the Satyam fraud, the 2009 election-day upper circuit, the March 2020 crash). The other 22, all 16-28% moves outside any look-back, are listed as "outside use".
- Compared with Yahoo's dividend history on 368 symbols (2010-2026), the amounts agree on 6,150 of 6,197 shared ex-dates. Yahoo has 113 ex-dates that NSE's records lack, about 2% of all dividends.
- Every member has a price bar on every trading day in both lists. The only exceptions are two gold-ETF-only special sessions (16 May 2010, 11 Nov 2012), when no stock traded.

**Known limits.**
- Demerger factors are estimates, and on a volatile ex-date the close-based fallback absorbs that day's market move (Siemens on 7 Apr 2025).
- Dividends missing from NSE's records are missing here.
- Prices are not dividend-adjusted. The simulator credits dividends separately, and `AdjClose` in the price files is a total-return series for reference only.

## Run checks

`cmd/research` stops a run on these universes if any trading day has the wrong member count, if a member has no price file, or if a member has no bar on a day the market traded. `-lag` must be 0: NSE announces every change weeks before it takes effect, so the exact dates carry no look-ahead.
