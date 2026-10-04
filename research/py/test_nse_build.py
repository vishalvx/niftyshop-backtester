#!/usr/bin/env python3
"""Tests for the press-release and corporate-action parsers in nse_build.py. Run: python3 -m unittest research/py/test_nse_build.py"""
import datetime as dt
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(__file__))
import nse_build as b  # noqa: E402


class Classify(unittest.TestCase):
    def check(self, subject, face, want):
        self.assertEqual(b.classify(subject, face), want, subject)

    def test_splits_and_bonuses(self):
        self.check("Face Value Split (Sub-Division) - From Rs 10/- Per Share To Rs 2/- Per Share", 2,
                   [("split", {"from": 10.0, "to": 2.0})])
        self.check("Fv Split Rs.10/- To Rs.2/", 2, [("split", {"from": 10.0, "to": 2.0})])
        self.check("Agm/Div-Rs.2/Fv Rs10tors5", 5, [("split", {"from": 10.0, "to": 5.0}), ("dividend", {"amount": 2.0})])
        self.check("Fv Spl-Rs10tore1/Bon-3:2", 1, [("bonus", {"new": 3, "held": 2}), ("split", {"from": 10.0, "to": 1.0})])
        self.check("Bonus 1:1/Div-Rs.5 Per Sh", 1, [("bonus", {"new": 1, "held": 1}), ("dividend", {"amount": 5.0})])
        self.check("Consolidation Of Equity Shares From Re 1 Per Share To Rs 10 Per Share", 10,
                   [("split", {"from": 1.0, "to": 10.0})])

    def test_rights(self):
        # 9:77 must not be read as 9:7, and a premium is added to the face value.
        self.check("Rights 9:77 Partly Paid @ Premium Rs 100/-", 10, [("rights", {"new": [9], "held": 77, "price": [110.0]})])
        self.check("Rights 1:10 @ Prem Rs 197/- Per Share", 10, [("rights", {"new": [1], "held": 10, "price": [207.0]})])
        self.check("Rights 1:6 @ Rs 1250/-", 10, [("rights", {"new": [1], "held": 6, "price": [1250.0]})])
        self.check("Rights - 4:25 Fully Paid Up Shares @ Premium Rs 500/- Per Share / 2:25 Partly Paid Up Shares @ "
                   "Premium Rs 605/- Per Share", 10, [("rights", {"new": [4, 2], "held": 25, "price": [510.0, 615.0]})])

    def test_dividends_and_demergers(self):
        self.check("Dividend - Rs 2.50 Per Share Plus Special Dividend - Rs 1 Per Share", 1, [("dividend", {"amount": 3.5})])
        self.check("AGM/Dividend - 25%", 10, [("dividend", {"amount": 2.5})])
        self.check("Interim Dividend - Re 0.75 Per Share (Purpose Revised)", 1, [("dividend", {"amount": 0.75})])
        self.check("Demerger", 1, [("demerger", {"explicit": True})])
        self.check("Scheme Of Demerger", 1, [("demerger", {"explicit": True})])
        self.check("Sch Of Arngmnt/Div-100% Purpose Revised", 1, [("demerger", {}), ("dividend", {"amount": 1.0})])
        self.check("Annual General Meeting", 1, [])
        self.check("Annual General Meeting / Dividend - Final Rs 22 + Special Rs 10", 5, [("dividend", {"amount": 32.0})])
        self.check("Dividend Final Rs 2.80 And Special Rs 1.65 Per Share", 1, [("dividend", {"amount": 4.45})])
        self.check("Interim Dividend - Rs 7 Per Share Special Dividend - Rs 3 Per Share", 10, [("dividend", {"amount": 10.0})])
        self.check("Bonus 1:2 And Dividend Rs.8/- Per Share", 1, [("bonus", {"new": 1, "held": 2}), ("dividend", {"amount": 8.0})])
        self.check("Dividend Rs.6/- Per Share And Face Value Split From Rs.2/- To Re.1/-", 1,
                   [("split", {"from": 2.0, "to": 1.0}), ("dividend", {"amount": 6.0})])
        self.check("Agm/Spl/Bon-1:1/Div-20%", 5, [("bonus", {"new": 1, "held": 1}), ("dividend", {"amount": 1.0})])
        self.check("Fv Split Rs.10/- To Rs.2/- /Div-50%", 2, [("split", {"from": 10.0, "to": 2.0}), ("dividend", {"amount": 5.0})])
        # bonus debentures and preference shares are not a share bonus
        self.check("Scheme Of Arrangement - Bonus Ncrps 4:1", 1, [("demerger", {"explicit": True})])
        self.check("Scheme Of Arrangement - Bonus Debentures 6:1", 5, [("demerger", {"explicit": True})])
        self.check("Sch Of Agmt- Bonus Deb1:1", 1, [("demerger", {"explicit": True})])
        self.check("Bonus Shares In The Ratio Of 1:1", 10, [("bonus", {"new": 1, "held": 1})])
        self.check("Spl-Rs10 To Rs2/Bonus-1:2", 2, [("bonus", {"new": 1, "held": 2}), ("split", {"from": 10.0, "to": 2.0})])


RELEASE = """
                      PRESS RELEASE
These changes shall become effective from September 30, 2026 (close of September 29, 2026).
\f    a) Nifty 50

     The following company is being excluded:
        Sr. No.          Company Name                         Symbol
          1     Wipro Ltd.                                    WIPRO

     The following company is being included:
        Sr. No.          Company Name                         Symbol
          1     BSE Ltd.                                      BSE

d) Nifty Next 50
     Note:
     1. Zomato Ltd. has been removed from Nifty Next 50 on account of its inclusion in
         Nifty 50 index
     The following companies are being included:
          1     Swiggy Ltd.                                   SWIGGY
b)  Nifty100 Liquid 15 Index
The following scrips are being included:
          1     Aurobindo Pharma Ltd.                         AUROPHARMA
c)  Nifty Midcap 150
The following scrips are being excluded:
          1     Allahabad Bank                                ALBK
(1)    S&P CNX Nifty Index
The following company is being excluded:
  Sr. No.    Company Name
  1          Satyam Computer Services Ltd.
"""


class ParseRelease(unittest.TestCase):
    def test_sections_rows_and_prose(self):
        self.assertEqual(list(b.parse_release(RELEASE)), [
            ("nifty50", "out", "Wipro Ltd.", "WIPRO"),
            ("nifty50", "in", "BSE Ltd.", "BSE"),
            ("niftymidcap150", "out", "Allahabad Bank", "ALBK"),
            ("nifty50", "out", "Satyam Computer Services Ltd.", ""),  # 2007-2011 releases name the company only
        ])

    def test_effective_date(self):
        self.assertEqual(b.effective_date("Replacements in indices w.e.f. March 30, 2026", ""), dt.date(2026, 3, 30))
        self.assertEqual(b.effective_date("Index Changes", RELEASE), dt.date(2026, 9, 30))
        self.assertEqual(b.effective_date("Change in Index w.e.f December 26th, 2007.", ""), dt.date(2007, 12, 26))


class Symbols(unittest.TestCase):
    def test_rename_chains_and_reuse(self):
        s = b.Symbols.__new__(b.Symbols)
        s.changes = {"SESAGOA": [(dt.date(2013, 10, 4), "SSLT")], "SSLT": [(dt.date(2015, 5, 7), "VEDL")],
                     "BAJAJAUTO": [(dt.date(2008, 3, 14), "BAJAJHLDNG")]}
        self.assertEqual(s.canonical("SESAGOA", dt.date(2012, 1, 1)), "VEDL")
        self.assertEqual(s.canonical("SSLT", dt.date(2014, 1, 1)), "VEDL")
        self.assertEqual(s.canonical("BAJAJAUTO", dt.date(2008, 1, 1)), "BAJAJHLDNG")
        self.assertEqual(s.canonical("VEDL", dt.date(2020, 1, 1)), "VEDL")
        self.assertEqual(s.history("VEDL"), [("VEDL", dt.date(2015, 5, 7), None), ("SSLT", dt.date(2013, 10, 4), dt.date(2015, 5, 7)),
                                             ("SESAGOA", None, dt.date(2013, 10, 4))])


if __name__ == "__main__":
    unittest.main()
