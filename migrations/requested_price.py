import psycopg2
import argparse
from dotenv import load_dotenv
import os
import time

# Load environment variables
load_dotenv(dotenv_path=os.path.join(os.getenv('PROJECTS_DIR'), 'slack-trading', '.env'))

# Database connection parameters (modify accordingly)
DB_PARAMS = {
    "dbname": os.getenv('POSTGRES_DB'),
    "user": os.getenv('POSTGRES_USER'),
    "password": os.getenv('POSTGRES_PASSWORD'),
    "host": os.getenv('POSTGRES_HOST'),
    "port": os.getenv('POSTGRES_PORT'),
}

def process_orders(live_run=False):
    # Connect to the database
    conn = psycopg2.connect(**DB_PARAMS)
    cursor = conn.cursor()

    # Query to get order records for a specific symbol
    query = f"SELECT id, price, requested_price FROM order_records;"
    cursor.execute(query, ())

    # Fetch all orders
    orders = cursor.fetchall()
    
    # List to store adjustments
    adjustments = []
    
    # dict order_id -> remaining_qty
    for order in orders:
        order_id, price, requested_price = order
        if price:
            adjustments.append((price, order_id))

    # Print adjustments
    for adj in adjustments:
        print(f"Adjustment: Order {adj[1]} -> Requested Price {adj[0]}")

    # If live run, commit changes (if applicable)
    if live_run:
        while len(adjustments) > 0:
            for adj in adjustments[:100]:
                update_query = "UPDATE order_records SET requested_price = %s, price = NULL WHERE id = %s;"
                cursor.execute(update_query, adj)
            conn.commit()
            adjustments = adjustments[100:]
            print("100 adjustments have been committed to the database.")
            time.sleep(1)
            
        print("Adjustments have been committed to the database.")
    else:
        print("Live run disabled. No database changes were made.")

    # Close database connection
    cursor.close()
    conn.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Process order adjustments.")
    parser.add_argument("--live-run", action="store_true", dest="live_run", help="Enable live mode (apply changes).")
    
    args = parser.parse_args()
    
    process_orders(live_run=args.live_run)
