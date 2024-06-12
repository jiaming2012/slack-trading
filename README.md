# slack-trading
A mock trading platform.

# Installation
1. Make sure docker is running
2. Install eventstoredb
``` bash
docker pull eventstore/eventstore:release-5.0.11
```

## Database
See README.md in infra project. Currently running on local Ubuntu box.

# Start Up
3. Make sure eventstoredb is running
``` bash
cd eventstoredb
docker-compose up
```
4. Run the interactive brokers daemon
``` bash
cd path/to/grodt/interactive-brokers/clientportal
./bin/run.sh root/conf.yaml
```
Open https://localhost:5000 to login


# Interactive brokers
Common instructions for working with interactive brokers
## Add a New Symbol to the Data Feed
1. Find the conid using postman

![Postman Request](interactive_brokers_fetch_new_symbol.png)


## Google Sheets Authentication
Navigate to console.cloud.google.com (jamal@yumyums.kitchen)

Click the navigation menu (hamburger menu - top left) -> APIs & Services -> Enabled APIs & services. Click the 'Credentials' tab.

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

# Heroku
If deploying to heroku, there are some gochas:

1. If the application does not have a web port, heroku will terminate the application. This can be prevented by running:
``` bash
heroku ps:scale worker=1
```