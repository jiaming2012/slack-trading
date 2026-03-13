from datetime import datetime, time, date
import pytz

# Define the NYSE timezone (Eastern Time)
eastern = pytz.timezone('US/Eastern')
utc = pytz.utc

# Define the NYSE trading hours in local time (Eastern Time)
nyse_open_time_local = time(9, 30)
nyse_close_time_local = time(16, 0)

# Get the current date
current_date = datetime.now().date()

# Create datetime objects for the opening and closing times in local time
nyse_open_datetime_local = datetime.combine(current_date, nyse_open_time_local)
nyse_close_datetime_local = datetime.combine(current_date, nyse_close_time_local)

# Localize the datetime objects to the Eastern Time zone
nyse_open_datetime_local = eastern.localize(nyse_open_datetime_local)
nyse_close_datetime_local = eastern.localize(nyse_close_datetime_local)

# Convert the localized datetime objects to UTC
nyse_open_datetime_utc = nyse_open_datetime_local.astimezone(utc)
nyse_close_datetime_utc = nyse_close_datetime_local.astimezone(utc)

def time_to_option_contract_expiration_in_minutes(expiration: date):
    # Get the current date and time in UTC
    current_datetime_utc = datetime.now().astimezone(utc)

    # Create a datetime object for the expiration date
    expiration_datetime_utc = datetime.combine(expiration, nyse_close_time_local)

    # Localize the expiration datetime object to the Eastern Time zone
    expiration_datetime_local = eastern.localize(expiration_datetime_utc)

    # Get the time difference between the current datetime and the expiration datetime
    time_difference = expiration_datetime_local - current_datetime_utc

    # Return the time difference in minutes
    return time_difference.total_seconds() / 60
    

if __name__ == "__main__":
    print("NYSE Opening Time (Local):", nyse_open_datetime_local.strftime('%Y-%m-%d %H:%M:%S %Z%z'))
    print("NYSE Closing Time (Local):", nyse_close_datetime_local.strftime('%Y-%m-%d %H:%M:%S %Z%z'))
    print("NYSE Opening Time (UTC):", nyse_open_datetime_utc.strftime('%Y-%m-%d %H:%M:%S %Z%z'))
    print("NYSE Closing Time (UTC):", nyse_close_datetime_utc.strftime('%Y-%m-%d %H:%M:%S %Z%z'))
