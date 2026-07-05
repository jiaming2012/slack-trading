#!/usr/bin/env python3
"""
Regression test for the covered call strategy demo.

Runs demo_covered_call.py against a live Go trading server and validates that
the output matches the reference values from a known-good run.

Requirements:
    - The Go trading server must be running on http://127.0.0.1:5051
    - The Polygon API key must be configured in the server environment
    - The grodt conda environment must be active

Usage:
    python test_demo_covered_call.py

Reference run (AAPL, 2025-01-01 to 2025-03-01, $200,000 balance):
    Final timestamp:  2025-02-28T15:30:00
    Final balance:    $198,916.41
    Final equity:     $201,359.58
    Realized P&L:     $-1,083.59
    Return:           -0.54%
    Total trades:     210
"""

import os
import re
import subprocess
import sys
import unittest


# Reference values from the known-good run
REFERENCE = {
    "symbol": "AAPL",
    "start": "2025-01-01",
    "end": "2025-03-01",
    "balance": 200_000,
    "final_timestamp": "2025-02-28T15:30:00",
    "final_balance": 198_916.41,
    "final_equity": 201_359.58,
    "realized_pnl": -1_083.59,
    "return_pct": -0.54,
    "total_trades": 210,
}

# How much tolerance to allow for floating point values.
# The strategy is deterministic given the same Polygon data, so we use tight
# tolerances. If Polygon's historical data changes this will break — that's
# intentional, as it indicates the strategy behaviour has diverged.
TOLERANCE_DOLLARS = 0.02   # ±$0.02
TOLERANCE_PERCENT = 0.01   # ±0.01%


def _parse_dollar(s: str) -> float:
    """Parse a dollar amount like '$198,916.41' or '$-1,083.59'."""
    return float(s.replace("$", "").replace(",", ""))


def _parse_percent(s: str) -> float:
    """Parse a percent value like '-0.54%'."""
    return float(s.replace("%", ""))


# This is an integration/regression test: it runs demos.demo_covered_call
# end-to-end against a *live* Go Twirp server on :5051 (plus a Polygon API key
# and a ~10-15 min run). Under a bare, headless `pytest tests/` with no server
# it can only fail with connection-refused. Mirroring the e2e smoke module, it
# self-skips unless its harness is explicitly declared present via
# DEMO_COVERED_CALL_HARNESS=1 -- the spec permits skips for tests whose declared
# harness is absent. The harness is `task test:demo-covered-call`
# (run_demo_covered_call_test.sh): it boots the server, sets the env var,
# runs this module, and tears the server down on every exit path.
@unittest.skipUnless(
    os.environ.get("DEMO_COVERED_CALL_HARNESS") == "1",
    "demo_covered_call regression runs under task test:demo-covered-call "
    "(live Twirp server on :5051 + Polygon)",
)
class TestDemoCoveredCall(unittest.TestCase):
    """Integration / regression test for demo_covered_call.py."""

    @classmethod
    def setUpClass(cls):
        """Run the demo script once and capture output."""
        # Package root is one level up from the tests/ directory
        package_root = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..')

        python_bin = sys.executable  # use whichever python is running this test

        cmd = [
            python_bin,
            "-m", "demos.demo_covered_call",
            "--symbol", REFERENCE["symbol"],
            "--start", REFERENCE["start"],
            "--end", REFERENCE["end"],
            "--balance", str(REFERENCE["balance"]),
        ]

        print(f"\n>>> Running: {' '.join(cmd)}")
        print(">>> This may take 10-15 minutes (Polygon API calls) ...")

        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            cwd=package_root,
            timeout=1800,  # 30-minute timeout
        )

        cls.stdout = result.stdout
        cls.stderr = result.stderr
        cls.returncode = result.returncode

        # The demo logs to stderr (loguru default)
        cls.output = cls.stderr

        if result.returncode != 0:
            print("STDOUT:", result.stdout[:2000])
            print("STDERR:", result.stderr[:2000])

    def test_exit_code(self):
        """Demo should exit cleanly."""
        self.assertEqual(self.returncode, 0, f"Demo exited with code {self.returncode}")

    def _extract(self, pattern: str) -> str:
        """Extract a value from the output using a regex pattern."""
        match = re.search(pattern, self.output)
        self.assertIsNotNone(match, f"Pattern not found in output: {pattern}")
        return match.group(1)

    def test_final_timestamp(self):
        ts = self._extract(r"Final timestamp:\s+(\S+)")
        # Strip timezone info for comparison (output may include -05:00)
        ts_no_tz = ts.split("-05:00")[0].split("+")[0]
        self.assertEqual(
            ts_no_tz,
            REFERENCE["final_timestamp"],
            f"Final timestamp mismatch: {ts}",
        )

    def test_final_balance(self):
        raw = self._extract(r"Final balance:\s+(\$[\d,.\-]+)")
        actual = _parse_dollar(raw)
        self.assertAlmostEqual(
            actual,
            REFERENCE["final_balance"],
            delta=TOLERANCE_DOLLARS,
            msg=f"Final balance: expected {REFERENCE['final_balance']}, got {actual}",
        )

    def test_final_equity(self):
        raw = self._extract(r"Final equity:\s+(\$[\d,.\-]+)")
        actual = _parse_dollar(raw)
        self.assertAlmostEqual(
            actual,
            REFERENCE["final_equity"],
            delta=TOLERANCE_DOLLARS,
            msg=f"Final equity: expected {REFERENCE['final_equity']}, got {actual}",
        )

    def test_realized_pnl(self):
        raw = self._extract(r"Realized P&L:\s+(\$[\d,.\-]+)")
        actual = _parse_dollar(raw)
        self.assertAlmostEqual(
            actual,
            REFERENCE["realized_pnl"],
            delta=TOLERANCE_DOLLARS,
            msg=f"Realized P&L: expected {REFERENCE['realized_pnl']}, got {actual}",
        )

    def test_return_percent(self):
        raw = self._extract(r"Return:\s+([\d.\-]+%)")
        actual = _parse_percent(raw)
        self.assertAlmostEqual(
            actual,
            REFERENCE["return_pct"],
            delta=TOLERANCE_PERCENT,
            msg=f"Return: expected {REFERENCE['return_pct']}%, got {actual}%",
        )

    def test_total_trades(self):
        raw = self._extract(r"Total trades placed:\s+(\d+)")
        actual = int(raw)
        self.assertEqual(
            actual,
            REFERENCE["total_trades"],
            f"Total trades: expected {REFERENCE['total_trades']}, got {actual}",
        )

    def test_playground_cleaned_up(self):
        """The demo should remove the playground from the server."""
        self.assertIn("Playground removed from server", self.output)


if __name__ == "__main__":
    unittest.main(verbosity=2)
