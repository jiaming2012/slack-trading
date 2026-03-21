import pandas_market_calendars as mcal
import argparse

# Set up argument parser
parser = argparse.ArgumentParser(description='Get NYSE calendar schedule for a given date range.')
parser.add_argument('start_date', type=str, help='Start date in YYYY-MM-DD format')
parser.add_argument('end_date', type=str, help='End date in YYYY-MM-DD format')

# Parse arguments
args = parser.parse_args()

# Create a calendar
nyse = mcal.get_calendar('NYSE')

# Show available calendars
print(nyse.schedule(start_date=args.start_date, end_date=args.end_date).to_csv())