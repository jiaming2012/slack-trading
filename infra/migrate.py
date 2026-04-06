"""Apply database migrations: extensions, sequences, analytics views.

Usage: python infra/migrate.py [--host HOST] [--port PORT] [--dbname DB] [--user USER] [--password PW]

Defaults are read from environment variables (POSTGRES_HOST, etc.) with
fallbacks matching the local docker-compose setup.

All statements are idempotent (IF NOT EXISTS / OR REPLACE) — safe to re-run.
"""

import argparse
import os
import sys
from pathlib import Path

try:
    import psycopg2
except ImportError:
    sys.exit("psycopg2-binary is required: pip install psycopg2-binary")

REPO_ROOT = Path(__file__).resolve().parent.parent

# SQL files to apply, in order
SQL_FILES = [
    REPO_ROOT / "src" / "go" / "backtester-api" / "db" / "init.sql",
    REPO_ROOT / "infra" / "analytics-schema.sql",
]


def _split_statements(sql: str) -> list[str]:
    """Split SQL into individual statements, respecting $$ dollar-quoted blocks."""
    statements = []
    current: list[str] = []
    in_dollar = False

    for line in sql.split("\n"):
        stripped = line.strip()
        if stripped.startswith("--") and not current:
            continue

        current.append(line)

        # Track $$ blocks (DO $$ ... $$;)
        if line.count("$$") % 2 == 1:
            in_dollar = not in_dollar

        if not in_dollar and ";" in line:
            stmt = "\n".join(current).strip()
            if stmt:
                statements.append(stmt)
            current = []

    if current:
        stmt = "\n".join(current).strip()
        if stmt:
            statements.append(stmt)

    return statements


def migrate(conn):
    conn.autocommit = True
    cur = conn.cursor()

    for path in SQL_FILES:
        if not path.exists():
            print(f"SKIP: {path.relative_to(REPO_ROOT)} (file not found)")
            continue

        print(f"\n--- Applying {path.relative_to(REPO_ROOT)} ---")
        sql = path.read_text()

        for stmt in _split_statements(sql):
            clean_lines = [l for l in stmt.split("\n") if l.strip() and not l.strip().startswith("--")]
            clean = " ".join(l.strip() for l in clean_lines)
            if not clean:
                continue
            try:
                cur.execute(stmt)
                label = clean[:72]
                print(f"  OK: {label}{'...' if len(clean) > 72 else ''}")
            except Exception as e:
                msg = str(e).splitlines()[0]
                label = clean[:50]
                print(f"  WARN: {msg}  [{label}]")

    cur.close()
    print("\nMigration complete.")


def main():
    parser = argparse.ArgumentParser(description="Apply database migrations")
    parser.add_argument("--host", default=os.getenv("POSTGRES_HOST", "localhost"))
    parser.add_argument("--port", type=int, default=int(os.getenv("POSTGRES_PORT", "5432")))
    parser.add_argument("--dbname", default=os.getenv("POSTGRES_DB", "playground"))
    parser.add_argument("--user", default=os.getenv("POSTGRES_USER", "grodt"))
    parser.add_argument("--password", default=os.getenv("POSTGRES_PASSWORD", "test747"))
    args = parser.parse_args()

    conn = psycopg2.connect(
        host=args.host,
        port=args.port,
        dbname=args.dbname,
        user=args.user,
        password=args.password,
    )
    try:
        migrate(conn)
    finally:
        conn.close()


if __name__ == "__main__":
    main()
