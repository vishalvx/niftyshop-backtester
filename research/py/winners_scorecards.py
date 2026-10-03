#!/usr/bin/env python3
"""Split `research score` outputs per variant and write one full-metric markdown scorecard (four accounting stages) each.
Usage: winners_scorecards.py <outdir> <label>=<scorefile> ..."""
import re, sys, os
src = open('research/py/scorecard_tables.py').read().split("for v in variants:")[0]
ns = {}
sys.argv_saved = sys.argv
sys.argv = [sys.argv[0]]
exec(src, ns)
parse, rows, cell = ns['parse'], ns['rows'], ns['cell']
out = sys_out = sys.argv_saved[1]
os.makedirs(out, exist_ok=True)
for spec in sys.argv_saved[2:]:
    label, path = spec.split('=', 1)
    txt = open(path).read().strip()
    blocks = re.split(r'\n(?==== )', txt)
    byvar = {}
    for b in blocks:
        m = re.match(r'=== (\S+) \| stage (\S+) \|', b)
        if m:
            byvar.setdefault(m[1], []).append(b)
    for v, bl in byvar.items():
        tmp = os.path.join(out, '.tmp.txt')
        open(tmp, 'w').write('\n'.join(bl))
        p = parse(tmp)
        stages = [s for s in ['gross', 'cost', 'tax-dated', 'tax-today'] if s in p]
        name = re.sub(r'[^A-Za-z0-9._-]+', '_', f'{label}__{v}')
        lines = [f'# {label}: {v}\n', 'Source: ' + path + '\n',
                 '| metric | ' + ' | '.join({'gross': 'before costs', 'cost': 'after costs', 'tax-dated': 'after costs + tax (rates by sale date)', 'tax-today': "after costs + tax (today's rates throughout)"}[s] for s in stages) + ' |',
                 '|---|' + '---|' * len(stages)]
        for nme, k in rows:
            lines.append(f'| {nme} | ' + ' | '.join(cell(p[s], k) for s in stages) + ' |')
        for b in bl:
            if 'stage cost' in b.split('\n')[0]:
                m = re.search(r'stress windows: (.*)', b)
                if m:
                    lines.append('\nStress windows (equity after costs; return and worst fall inside the window): ' + m[1])
        open(os.path.join(out, name + '.md'), 'w').write('\n'.join(lines) + '\n')
        os.remove(tmp)
