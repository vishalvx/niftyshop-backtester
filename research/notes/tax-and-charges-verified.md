# Indian equity tax and charges, dated and sourced (verified 2026-10-01 by a research sub-agent)

Dates inclusive. Source keys are listed at the bottom. Items tagged "secondary" in section B could not be confirmed
from a primary document. Everything coded in `internal/costs/rates.go` comes from this table.

## A. Master table

| Item | Rate | From | To | Rule / key quote | Src |
|---|---|---|---|---|---|
| STCG s.111A (STT-paid listed shares, equity fund units) | 10% | 2004 | 2008-03-31 | by transfer date; "raised from ten per cent to fifteen per cent" | S5 |
| STCG | 15% | 2008-04-01 | 2024-07-22 | the memo says it takes effect "1st April, 2009 ... assessment year 2009-10"; AY2009-10 is FY2008-09 income, so 15% hits sales from 2008-04-01 | S5, S6 |
| STCG | 20% | 2024-07-23 | open | "twenty per cent. for any transfer which takes place on or after the 23rd day of July, 2024" | S1 s.29 |
| LTCG, old | exempt (s.10(38)) | 2004 | 2018-03-31 | "exempt from income-tax under clause (38) of section 10" | S4 |
| LTCG s.112A | 10% above Rs 1 lakh per FY | 2018-04-01 | 2024-07-22 | needs STT on buy and sell; cost grandfathered to the highest price on 31-Jan-2018 | S4 |
| LTCG s.112A | 12.5% above Rs 1.25 lakh per FY | 2024-07-23 | open | "twelve and one-half per cent. for any transfer which takes place on or after the 23rd day of July, 2024"; one Rs 1.25 lakh limit "on aggregate" across both rates in FY2024-25 | S1 s.31 |
| Holding test | short term = 12 months or less | 2007 | open | "for listed securities, it is one year" (2024); 2007-2024 secondary only | S2 |
| Cess | 2% to 2007-03-31; 3% (2%+1%) to 2018-03-31; 4% after | | open | "four per cent. of income tax including surcharge" | S4, S5, S9 |
| Surcharge | applies to 111A/112A tax, capped at 15% | | open | slabs 10% (Rs 50L-1Cr), 15% above | S1 |
| Losses | STCL vs STCG or LTCG; LTCL vs LTCG only; carry forward 8 AYs | | open | "long-term capital loss can only be adjusted with any long-term capital gains only" | S16 |
| STT, delivery shares | 0.125% buyer and seller | 2006-06-01 | 2012-06-30 | memo table "Existing Rates 0.125" | S6 |
| STT, delivery shares | 0.1% buyer and seller | 2012-07-01 | open | "effective from the 1st day of July, 2012" | S6 |
| STT Budget changes | 2024: options to 0.1%, futures to 0.02% (2024-10-01); 2025: none; 2026: options 0.15%, futures 0.05% (2026-04-01); delivery unchanged | | | "0.1 per cent (No Change)" | S8, S9, S10 |
| NSE cash charge, per side | 0.00325% | ~2009 | 2020-12-31 | slabs ran 3.00-3.25 | S11a |
| NSE cash charge | 0.00345% | 2021-01-01 | 2023-03-31 | IPFT top-up | S11a |
| NSE cash charge | 0.00325% + IPFT Rs 10/crore | 2023-04-01 | 2024-03-31 | | S11b |
| NSE cash charge | 0.00322% + IPFT | 2024-04-01 | 2024-09-30 | | S11c |
| NSE cash charge | 0.00297% flat + IPFT | 2024-10-01 | 2026-02-28 | SEBI uniform-charge rule | S11d |
| NSE cash charge | 0.0030699% + IPFT Rs 0.01/crore | 2026-03-01 | open | | S11e |
| SEBI turnover fee | Rs 20/crore | pre-2017 | 2016-12 | | S12 |
| SEBI turnover fee | Rs 15/crore | ~2017 | 2019-03-31 | effective date not found | S12 |
| SEBI turnover fee | Rs 10/crore | 2019-04-01 | open | "0.00010 per cent of his turnover" | S12 |
| Stamp duty, buy side | about 0.01%, varies by state | | 2020-06-30 | broker page | S14 |
| Stamp duty, buy side | 0.015% uniform | 2020-07-01 | open | NSE page | S13 |
| Service tax / GST | 12.36% to 2009-02-23; 10.30% to 2012-03-31; 12.36% to 2015-05-31; 14% to 2015-11-14; 14.5% to 2016-05-31; 15% to 2017-06-30; GST 18% after | | open | applies to brokerage + exchange + SEBI fees, not STT or stamp | S15 |
| DP charge per scrip per sell day | Rs 13.5 (2016-2019) to Rs 15.34 now | | | depository part is tariff, broker part is broker-set | S17 |
| Brokerage | Zerodha delivery: lower of 0.1% or Rs 20 until Dec 2015, then zero; full-service about 0.29-0.30% now | | | broker pages, not statutory | S18 |
| ETF (e.g. NIFTYBEES) | same 111A/112A rates and 12-month test; STT as shares to 2013-05-31, then buy nil, sell 0.001% | 2013-06-01 | open | | S7, S8 |
| Dividends | tax-free to the holder (company paid 15% DDT) | | 2020-03-31 | s.10(34) | S19 |
| Dividends | 10% above Rs 10 lakh | FY2016-17 | FY2019-20 | | S19 |
| Dividends | slab rate + surcharge (capped 15%) + cess | 2020-04-01 | open | | S19 |
| TDS on dividend | 10%; none up to Rs 10,000 a year (Rs 5,000 before) | 2025-04-01 | open | | S10 |

All-in capital-gains rate for a resident individual = rate x (1 + surcharge) x (1 + cess):

| Sale date | STCG, cess only | STCG, 15% surcharge | LTCG, cess only | LTCG, 15% surcharge |
|---|---|---|---|---|
| 2008-04 to 2018-03 | 15.45% | up to 17.77% | exempt | exempt |
| 2018-04 to 2024-07-22 | 15.6% | 17.94% | 10.4% | 11.96% |
| From 2024-07-23 | 20.8% | 23.92% | 13.0% | 14.95% |

Surcharge needs total income above Rs 50 lakh, so "cess only" is the default used in the study.

## B. Confidence and gaps

- Conflict, STT date: Wikipedia says the delivery cut came in the "2013 budget"; the Budget memo (S6) says 1 Jul 2012. The 2013 change was the ETF cut (S7).
- Conflict, STCG date: the 2008 memo says "1st April, 2009", which is the assessment-year start; applying 15% from FY2009-10 would be wrong.
- Conflict, SEBI fee date: the SEBI notification says 1 Apr 2019; an exchange notice says 3 Apr 2019.
- Secondary only: the 12-month holding test for 2007-2024; STT start 1 Jun 2006; the extra 1% cess from 2007; the 1 Jun 2015 service-tax date; the FY2019-20 start of the 15% surcharge cap.
- Individual surcharge history: verified only for FY2007-08 and FY2011-12/2012-13; other years from memory.
- NSE charges before 2021: only the end state is primary; the ~2009 start comes from a news snippet.
- SEBI fee: 2007-2016 levels and the Rs 15 effective date not found.
- Stamp duty before July 2020: no state table obtained; buy/sell split unverified.
- DP charge 2008-2015: no figure found. Historic full-service brokerage of 0.3-0.5%: only current pages seen.
- ETF stamp duty: the NSE table says "security other than debenture"; no ETF-specific confirmation.
- The Income-tax Act 2025 applies from 2026-04-01 and renumbers the capital-gains sections (196/197/198). No rate change found in S9 or S10.

## Source key

- S1 https://egazette.gov.in/WriteReadData/2024/256436.pdf (Finance (No.2) Act 2024)
- S2 https://www.pib.gov.in/PressReleaseIframePage.aspx?PRID=2036604 (CBDT FAQ)
- S4 https://www.indiabudget.gov.in/budget2018-2019/ub2018-19/memo/memo.pdf
- S5 https://www.indiabudget.gov.in/budget_archive/ub2008-09/mem/mem1.pdf
- S6 https://www.indiabudget.gov.in/budget2012-2013/ub2012-13/mem/mem1.pdf
- S7 https://www.indiabudget.gov.in/budget2013-2014/ub2013-14/mem/mem1.pdf
- S8 https://nsearchives.nseindia.com/content/circulars/FATAX63809.pdf
- S9 https://www.indiabudget.gov.in/doc/memo.pdf (Finance Bill 2026)
- S10 https://www.indiabudget.gov.in/budget2025-26/doc/memo.pdf
- S11 https://nsearchives.nseindia.com/content/circulars/FA46730.pdf (a); FA56129 (b); FA61137 (c); FA64232 (d); FA73061 (e)
- S12 https://www.sebi.gov.in/sebi_data/meetingfiles/mar-2019/1553244864312_1.pdf ; https://www.cse-india.com/upload/cse_notice/REVISED_SEBI_TURNOVER_FEES.htm
- S13 https://www.nseindia.com/static/invest/first-time-investor-stamp-duty-charges-taxes
- S14 https://zerodha.com/z-connect/general/uniform-stamp-duty (broker page)
- S15 https://www.indiabudget.gov.in/budget2012-2013/ub2012-13/cen/dojstru2.pdf ; budget2015-2016 and budget2016-2017 dojstru2.pdf ; https://pib.gov.in/newsite/printrelease.aspx?relid=130308 ; https://cbic-gst.gov.in/hindi/pdf/central-tax-rate/Notification11-CGST.pdf
- S16 https://www.incometax.gov.in/iec/foportal/sites/default/files/2021-05/Instructions_ITR2_AY2020_21_V1_0.pdf
- S17 https://zerodha.com/charges ; web.archive.org snapshots 2016 and 2019 (broker pages)
- S18 zerodha.com/z-connect/business-updates/big-savings-with-zero-brokerage ; icicidirect.com/brokerage ; sharekhan.com/pricing (broker pages)
- S19 https://www.indiabudget.gov.in/budget2020-21/doc/memo.pdf ; https://www.indiabudget.gov.in/budget2016-2017/ub2016-17/memo/mem1.pdf
