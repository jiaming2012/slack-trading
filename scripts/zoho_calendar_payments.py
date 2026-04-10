#!/usr/bin/env python3
"""
Fetch and display upcoming events from a Zoho Calendar "Payments" calendar.

Required environment variables (or set in .env file):
    ZOHO_CLIENT_ID       - OAuth2 client ID from https://api-console.zoho.com/
    ZOHO_CLIENT_SECRET   - OAuth2 client secret
    ZOHO_REFRESH_TOKEN   - OAuth2 refresh token (offline access)
    ZOHO_CALENDAR_ID     - Unique ID of the Payments calendar

Usage:
    python scripts/zoho_calendar_payments.py              # Next 7 days (default)
    python scripts/zoho_calendar_payments.py --days 14    # Next 14 days
    python scripts/zoho_calendar_payments.py --days 1     # Today only
"""

import argparse
import json
import os
import sys
from datetime import datetime, timedelta

try:
    import requests
except ImportError:
    print("Error: 'requests' package is required. Install with: pip install requests", file=sys.stderr)
    sys.exit(1)

try:
    from dotenv import load_dotenv
    load_dotenv()
except ImportError:
    # python-dotenv is optional; env vars can be set directly
    pass


ZOHO_TOKEN_URL = "https://accounts.zoho.com/oauth/v2/token"
ZOHO_CALENDAR_API_BASE = "https://calendar.zoho.com/api/v1"


def get_required_env(name):
    """Get a required environment variable or exit with a clear error."""
    value = os.environ.get(name)
    if not value:
        print(
            f"Error: Missing required environment variable: {name}\n"
            f"Set it in your shell or in a .env file.\n"
            f"See script docstring for all required variables.",
            file=sys.stderr,
        )
        sys.exit(1)
    return value


def get_access_token(client_id, client_secret, refresh_token):
    """Exchange a refresh token for an access token via Zoho OAuth2."""
    payload = {
        "grant_type": "refresh_token",
        "client_id": client_id,
        "client_secret": client_secret,
        "refresh_token": refresh_token,
    }

    try:
        resp = requests.post(ZOHO_TOKEN_URL, data=payload, timeout=15)
    except requests.RequestException as e:
        print(f"Error: Failed to connect to Zoho auth server: {e}", file=sys.stderr)
        sys.exit(1)

    if resp.status_code != 200:
        print(
            f"Error: Zoho auth returned HTTP {resp.status_code}\n"
            f"Response: {resp.text}",
            file=sys.stderr,
        )
        sys.exit(1)

    data = resp.json()
    if "access_token" not in data:
        print(
            f"Error: No access_token in Zoho auth response.\n"
            f"Response: {json.dumps(data, indent=2)}",
            file=sys.stderr,
        )
        sys.exit(1)

    return data["access_token"]


def fetch_events(access_token, calendar_id, start_date, end_date):
    """Fetch events from a Zoho Calendar within a date range."""
    url = f"{ZOHO_CALENDAR_API_BASE}/calendars/{calendar_id}/events"

    headers = {
        "Authorization": f"Zoho-oauthtoken {access_token}",
    }

    params = {
        "range": json.dumps({
            "start": start_date.strftime("%Y%m%dT000000Z"),
            "end": end_date.strftime("%Y%m%dT235959Z"),
        }),
    }

    try:
        resp = requests.get(url, headers=headers, params=params, timeout=30)
    except requests.RequestException as e:
        print(f"Error: Failed to connect to Zoho Calendar API: {e}", file=sys.stderr)
        sys.exit(1)

    if resp.status_code == 401:
        print(
            "Error: Authentication failed (401). Your refresh token may be expired.\n"
            "Generate a new one at https://api-console.zoho.com/",
            file=sys.stderr,
        )
        sys.exit(1)

    if resp.status_code == 404:
        print(
            f"Error: Calendar not found (404). Check ZOHO_CALENDAR_ID: {calendar_id}",
            file=sys.stderr,
        )
        sys.exit(1)

    if resp.status_code != 200:
        print(
            f"Error: Zoho Calendar API returned HTTP {resp.status_code}\n"
            f"Response: {resp.text}",
            file=sys.stderr,
        )
        sys.exit(1)

    data = resp.json()
    return data.get("events", [])


def parse_zoho_datetime(dt_str):
    """Parse a Zoho datetime string into a Python datetime.

    Zoho returns dates in formats like '20260410T140000Z' or '20260410T140000+0530'.
    Falls back to returning the raw string if parsing fails.
    """
    for fmt in ("%Y%m%dT%H%M%SZ", "%Y%m%dT%H%M%S%z", "%Y-%m-%dT%H:%M:%S%z", "%Y-%m-%dT%H:%M:%SZ"):
        try:
            return datetime.strptime(dt_str, fmt)
        except (ValueError, TypeError):
            continue
    return dt_str


def format_event(event):
    """Format a single calendar event for terminal display."""
    title = event.get("title", "(no title)")

    # Parse start/end times
    start_raw = event.get("dateandtime", {}).get("start", "")
    end_raw = event.get("dateandtime", {}).get("end", "")
    is_all_day = event.get("isallday", False)

    start_dt = parse_zoho_datetime(start_raw)
    end_dt = parse_zoho_datetime(end_raw)

    if is_all_day:
        if isinstance(start_dt, datetime):
            date_str = start_dt.strftime("%a %b %d, %Y")
        else:
            date_str = str(start_raw)
        time_str = "All day"
    else:
        if isinstance(start_dt, datetime):
            date_str = start_dt.strftime("%a %b %d, %Y")
            start_time = start_dt.strftime("%I:%M %p")
            if isinstance(end_dt, datetime):
                end_time = end_dt.strftime("%I:%M %p")
                time_str = f"{start_time} - {end_time}"
            else:
                time_str = start_time
        else:
            date_str = str(start_raw)
            time_str = ""

    # Description (truncated)
    description = event.get("description", "")
    if description and len(description) > 100:
        description = description[:97] + "..."

    return {
        "title": title,
        "date": date_str,
        "time": time_str,
        "description": description,
    }


def display_events(events, days):
    """Display formatted events to stdout."""
    if not events:
        print(f"\nNo upcoming events found in the next {days} day(s).")
        return

    print(f"\n{'=' * 60}")
    print(f"  Payments Calendar - Next {days} Day(s)")
    print(f"  {datetime.now().strftime('%a %b %d, %Y')}")
    print(f"{'=' * 60}\n")

    for i, event in enumerate(events, 1):
        fmt = format_event(event)
        print(f"  [{i}] {fmt['title']}")
        print(f"      Date: {fmt['date']}")
        if fmt["time"]:
            print(f"      Time: {fmt['time']}")
        if fmt["description"]:
            print(f"      Note: {fmt['description']}")
        print()

    print(f"{'=' * 60}")
    print(f"  Total: {len(events)} event(s)")
    print(f"{'=' * 60}")


def main():
    parser = argparse.ArgumentParser(
        description="Fetch and display upcoming events from a Zoho Calendar 'Payments' calendar.",
        epilog=(
            "Required environment variables:\n"
            "  ZOHO_CLIENT_ID       OAuth2 client ID\n"
            "  ZOHO_CLIENT_SECRET   OAuth2 client secret\n"
            "  ZOHO_REFRESH_TOKEN   OAuth2 refresh token\n"
            "  ZOHO_CALENDAR_ID     Payments calendar ID\n"
            "\n"
            "Set these in your shell or in a .env file in the project root."
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--days",
        type=int,
        default=7,
        help="Number of days to look ahead (default: 7)",
    )
    args = parser.parse_args()

    if args.days < 1:
        print("Error: --days must be at least 1.", file=sys.stderr)
        sys.exit(1)

    # Load configuration
    client_id = get_required_env("ZOHO_CLIENT_ID")
    client_secret = get_required_env("ZOHO_CLIENT_SECRET")
    refresh_token = get_required_env("ZOHO_REFRESH_TOKEN")
    calendar_id = get_required_env("ZOHO_CALENDAR_ID")

    # Authenticate
    print("Authenticating with Zoho...", end=" ", flush=True)
    access_token = get_access_token(client_id, client_secret, refresh_token)
    print("OK")

    # Fetch events
    start_date = datetime.utcnow()
    end_date = start_date + timedelta(days=args.days)

    print(f"Fetching events from {start_date.strftime('%Y-%m-%d')} to {end_date.strftime('%Y-%m-%d')}...", end=" ", flush=True)
    events = fetch_events(access_token, calendar_id, start_date, end_date)
    print("OK")

    # Display
    display_events(events, args.days)


if __name__ == "__main__":
    main()
