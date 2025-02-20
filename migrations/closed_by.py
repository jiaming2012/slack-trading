import psycopg2
import argparse
from dotenv import load_dotenv
import os

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

def process_orders(symbol, playground_id, live_run=False):
    # Connect to the database
    conn = psycopg2.connect(**DB_PARAMS)
    cursor = conn.cursor()

    # Query to get order records for a specific symbol
    query = f"SELECT id, symbol, quantity, side FROM order_records WHERE symbol = %s and status = 'filled' and playground_id = %s ORDER BY timestamp ASC;"
    cursor.execute(query, (symbol, playground_id))

    # Fetch all orders
    orders = cursor.fetchall()
    
    # List to store adjustments
    adjustments = []
    
    # dict order_id -> remaining_qty
    remaining_qty = {}
    for order in orders:
        order_id, symbol, quantity, side = order
        remaining_qty[order_id] = quantity

    # Iterate through orders in reverse
    j = len(orders) - 1
    while j >= 0:
        order_id, symbol, quantity, side = orders[j]
        
        if remaining_qty[order_id] == 0:
            j -= 1
            continue

        # Condition: If it's a buy-to-cover or sell order
        if side in ("buy_to_cover", "sell"):            
            closing_qty = 0  # Track how much needs to be closed
            
            trade_query = f"SELECT id FROM trade_records WHERE order_id = %s;"
            cursor.execute(trade_query, (order_id,))
            
            trade_id = cursor.fetchone()[0]

            # Iterate in reverse again to match closing orders
            for i in range(j, -1, -1):
                prev_order_id, prev_symbol, prev_quantity, prev_order_type = orders[i]
                # print('close id: ', prev_order_id)
                # If the previous order is a sell-short or buy
                if remaining_qty[prev_order_id] == 0:
                    continue
                
                if prev_order_type in ("sell_short", "buy"):
                    qty_to_close = min(remaining_qty[prev_order_id], quantity - closing_qty)
                    
                    # Update remaining quantity
                    remaining_qty[prev_order_id] -= qty_to_close
                        
                    # Update closing quantity
                    closing_qty += qty_to_close

                    # Append adjustment when matching order found
                    adjustments.append((prev_order_id, trade_id))
                    
                    if closing_qty == prev_quantity:
                        # Remove the matched order from the list
                        o = orders.pop(i)
                        j -= 1
                    elif closing_qty > quantity:
                        for adj in adjustments:
                            print(f"Adjustment: Order {adj[0]} -> Trade {adj[1]}")
                        raise ValueError("Invalid quantity: expected closing quantity to equal order quantity.")
                    
                    break
            
        j -= 1

    # Print adjustments
    for adj in adjustments:
        print(f"Adjustment: Order {adj[0]} -> Trade {adj[1]}")
        
    for order_id, qty in remaining_qty.items():
        if qty < 0:
            raise ValueError(f"Invalid quantity: order {order_id} has negative quantity.")

    # If live run, commit changes (if applicable)
    if live_run:
        for adj in adjustments:
            update_query = "INSERT INTO trade_closed_by (order_record_id, trade_record_id) VALUES (%s, %s);"
            cursor.execute(update_query, adj)
        conn.commit()
        print("Adjustments have been committed to the database.")
    else:
        print("Live run disabled. No database changes were made.")

    # Close database connection
    cursor.close()
    conn.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Process order adjustments.")
    parser.add_argument("--symbol", required=True, help="Stock symbol to process orders for.")
    parser.add_argument("--playground-id", required=True, help="Playground ID to filter orders.")
    parser.add_argument("--live-run", action="store_true", dest="live_run", help="Enable live mode (apply changes).")
    
    args = parser.parse_args()
    
    process_orders(args.symbol, args.playground_id, live_run=args.live_run)
