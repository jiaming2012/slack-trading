# slack-trading
A mock trading platform.

# Telemetry
Currently using the free tier of telemetry cloud: https://grafana.com/orgs/jac475. Used the following guide to set up: https://grafana.com/docs/grafana-cloud/monitor-applications/application-observability/setup/quickstart/go/

## See Telemetry Data
1. Launch Grafana
2. Click "Data Sources" in the side menu
3. Click grafanacloud-jac475-traces -> "Explore"
4. Select: Query type -> "Search"

# Running locally
## Install golang and python
Both golang:1.20 and python:3.7.9 are required.

If installing on ubuntu:
``` bash
sudo apt-get install python3.8-venv
```

## Initiate your python env
``` bash
cd path/to/slack-trading/src/cmd/stats
python3 -m venv $PROJECTS_DIR/slack-trading/src/cmd/stats/env
$PROJECTS_DIR/slack-trading/src/cmd/stats/env/bin/pip install -r src/cmd/stats/requirements.txt
```

# Dockerfile
## Prod
We currently host the base 1 and base 2 images at vultr. In order to access them, log in with:
``` bash
docker login https://ewr.vultrcr.com/base1 -u $VULTR_USER -p $VULTR_PASS
```

`$VULTR_USER` and `VULTR_PASS` can both be foound in the vultr dashboard, under Container Registry.

## Dev
As such the main program can be built with commands:
``` bash
docker build -f Dockerfile.base -t grodt-base-image .
docker build -f Dockerfile.base2 -t grodt-base-image-2 .
docker build -f Dockerfile.dev -t grodt-main .
```

## Prod
We are currently using heroku for prod. In order to upload new base images:
``` bash
heroku container:login
docker tag <image> registry.heroku.com/<app>/<process-type>
docker push registry.heroku.com/<app>/<process-type>
heroku container:release web -a <app>
```

For example, *app* is `grodt` and *process-type* is `web`

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