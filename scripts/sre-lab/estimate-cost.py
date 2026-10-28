#!/usr/bin/env python3
"""Recalculate the review estimate. No AWS calls or account billing reads."""
from decimal import Decimal, ROUND_UP
import json
from pathlib import Path

plan = json.loads((Path(__file__).resolve().parents[2] / 'docs/sre/first-exercise-cost.json').read_text())
subtotal = sum(Decimal(row['quantity']) * Decimal(row['unit_rate_usd']) for row in plan['items'])
envelope = Decimal(plan['pretax_envelope_usd'])
if subtotal > envelope:
    raise SystemExit('The itemized estimate exceeds its reviewed envelope; re-review before launch.')
estimate = (envelope * (1 + Decimal(plan['tax_allowance_fraction']))).quantize(Decimal('0.01'), rounding=ROUND_UP)
contingency = Decimal(plan['extra_contingency_usd'])
headroom = Decimal(plan['budget_usd']) - estimate - contingency
if headroom < 0:
    raise SystemExit('Estimated total plus contingency exceeds the USD 50 budget.')
print(f'Itemized subtotal: USD {subtotal:.3f}; rounded pretax envelope: USD {envelope:.2f}.')
print(f'Estimated total including tax allowance: USD {estimate:.2f}.')
print(f'Including extra contingency: USD {estimate+contingency:.2f}; unused budget headroom: USD {headroom:.2f}.')
print('Existing account spend is not included or verified; no AWS calls were made.')
