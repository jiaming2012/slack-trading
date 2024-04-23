#!/bin/bash

export STOCK_QUOTES_URL="https://api.tradier.com/v1/markets/quotes"
export MARKET_CALENDAR_URL="https://api.tradier.com/v1/markets/calendar"
export OPTION_CHAIN_URL="https://api.tradier.com/v1/markets/options/chains"
export TRADIER_ACCOUNT_ID="6YA49543"
export TRADIER_BEARER_TOKEN="hgC2puSa7rukG5fYlAwOmDpihkhf"
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/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"
export TRADIER_ORDERS_URL_TEMPLATE="https://api.tradier.com/v1/accounts/%s/orders"
export EVENTSTOREDB_URL="esdb://us.loclx.io:21133?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000"

go run $PROJECTS_DIR/grodt/slack-trading/src/eventticks/main.go