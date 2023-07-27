# slack-trading
A mock trading platform.

## Google Sheets Authentication
To authenticate, create a service account on Google Cloud. Under **Keys**, select "Add Key" -> "Create new key". Download and base64 the JSON credentials file, and set the environment variable `KEY_JSON_BASE64` to base64 string.

# Hosting
The app is hosted on heroku

## Logs
Logs can be found via command:
``` bash
cd path/to/slack-trading
heroku logs
```

# Slack
UI is administered via slack. Admin page can be found here: https://api.slack.com/apps/A03C4E2TA6M

Events are sent to https://api.slack.com/apps/A03C4E2TA6M/event-subscriptions?