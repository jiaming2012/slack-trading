# slack-trading
A mock trading platform.

# DevOps

## Delete all playgrounds
During initial development, it has been useful to soft delete a set of existing playgrounds, with scripts below:
``` sql
-- mark all playgrounds as deleted
update playground_sessions set deleted_at = NOW() where environment != 'reconcile' and deleted_at is null ;

-- mark order records as deleted
update order_records as ord  
  set deleted_at = NOW()
  from playground_sessions as ps
  where ps.id = ord.playground_id
    and ps.deleted_at is not null ;

-- mark trade records as deleted
update trade_records as tr
  set deleted_at = NOW()
  from order_records as orec
  where tr.order_id = orec.id
    and orec.deleted_at is not null ;
```

## Installation
Crane: managing remote images in Vultr
``` bash
brew install crane
```

# Development
We use python's bump2version for managing the app version.

## Anaconda
We use anaconda for managing dependencies, instead of pip:
``` bash
conda deactivate
conda activate grodt
```

Setup interpreter
1. Find anaconda home directory:
``` bash
conda info | grep 'base environment'
```

2. Set env variable
``` bash
export ANACONDA_HOME="path/to/environment"
```

3. Set symbolic link for pm2 (on dev machines)
``` bash
cd ${PROJECTS_DIR}/slack-trading
ln -s $ANACONDA_HOME anaconda
```

### Update conda env
add to taskfile
``` bash
conda env update --file conda-env.yaml --prune
```

## PM2
PM2 is used for deploying client side.

Start all python scripts:
``` bash
pm2 start trading_engine.config.js --env production
```

## Migrations
Currently, no migrations framework has been chosen an scripts are used if database migrations are needed. The script does a dry-run by default. Running a second time with `--live-run` will apply the migrations.

See examples running `closed_by.py` below:

``` bash
(env) ➜  migrations git:(main) ✗ python closed_by.py --symbol META --playground-id "c9fed5c5-4c2c-4f62-8331-780df11cb61a"  
```

OUTPUT:
```
Adjustment: Order 465 -> Trade 145
Adjustment: Order 464 -> Trade 146
Adjustment: Order 458 -> Trade 142
Adjustment: Order 427 -> Trade 117
Adjustment: Order 404 -> Trade 101
Adjustment: Order 396 -> Trade 98
Adjustment: Order 377 -> Trade 86
Adjustment: Order 376 -> Trade 85
Adjustment: Order 365 -> Trade 70
Adjustment: Order 337 -> Trade 59
Live run disabled. No database changes were made.
```

Run live:
``` bash
(env) ➜  migrations git:(main) ✗ python closed_by.py --symbol META --playground-id "c9fed5c5-4c2c-4f62-8331-780df11cb61a" --live-run
```

OUTPUT:
```
Adjustment: Order 465 -> Trade 145
Adjustment: Order 464 -> Trade 146
Adjustment: Order 458 -> Trade 142
Adjustment: Order 427 -> Trade 117
Adjustment: Order 404 -> Trade 101
Adjustment: Order 396 -> Trade 98
Adjustment: Order 377 -> Trade 86
Adjustment: Order 376 -> Trade 85
Adjustment: Order 365 -> Trade 70
Adjustment: Order 337 -> Trade 59
Adjustments have been committed to the database.
```

## Taskfile
We use taskfile as our build tool.

#### On Mac
``` bash
brew install go-task
```

#### On Linux
``` bash
sudo snap install task --classic
```

You can list all commands with:
``` bash
task list
```

# Kubernetes issue
Here were some problems that needed to overcome when running in prod:
1. disk space full
solution:
ssh onto each node, run:
``` bash
crictl rmi --prune
```

2. Removing pvc
solution:
a. Remove the finializers first
``` bash
kubectl patch pvc <pvc-name> -n <namespace> --type=json -p '[{"op": "remove", "path": "/metadata/finalizers"}]'
```

3. Add the metrics server
``` bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch deployment metrics-server -n kube-system --type='json' -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-", "value":"--kubelet-insecure-tls"}]'

```

4. Removed the cpu limits on grodt deployment. You cannot add any deployment that you wish.

5. Create kube dashboard
``` bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml
kubectl create serviceaccount dashboard-admin -n kubernetes-dashboard
kubectl create clusterrolebinding dashboard-admin-binding \
  --clusterrole=cluster-admin \
  --serviceaccount=kubernetes-dashboard:dashboard-admin
```
Get the admin token to log in:
``` bash
kubectl -n kubernetes-dashboard create token dashboard-admin
```

# Twirp
We use twirp for grpc communication over http.

# Indicators
We use pandas-ta for indicators

## Installation
Mac
``` bash
brew install ta-lib  
conda env create -f conda-env.yaml
```

Ubuntu: visit https://docs.conda.io/projects/conda/en/stable/user-guide/install/rpm-debian.html

## Run the ML client
``` bash
cd ${PROJECTS_DIR}/slack-trading
./cmd/backtester/venv/bin/python ./cmd/backtester/proximal_policy_optimization_v13.py --symbol COIN --start-date 2024-09-03 --end-date 2024-11-13 --host http://127.0.0.1:5051
```

## Plot playground trades
``` bash
cd ${PROJECTS_DIR}/slack-trading
MY_PLAYGROUND="05a9b2ea-3fd5-414c-bf77-b73a73bb0d69"
./src/cmd/stats/env/bin/python ${PROJECTS_DIR}/slack-trading/src/cmd/stats/plot_playground.py --playground-id ${MY_PLAYGROUND} --host http://localhost:8080
```

## Compile protobuf file
``` bash
cd ${PROJECTS_DIR}/slack-trading
source src/cmd/stats/env/bin/activate
protoc --go_out=. --python_out=./src/cmd/stats --twirp_out=. --twirpy_out=./src/cmd/stats src/playground.proto
 mv ${PROJECTS_DIR}/slack-trading/src/cmd/stats/src/playground_pb2.py ${PROJECTS_DIR}/slack-trading/src/cmd/stats/rpc
 mv ${PROJECTS_DIR}/slack-trading/src/cmd/stats/src/playground_twirp.py ${PROJECTS_DIR}/slack-trading/src/cmd/stats/rpc
rmdir ${PROJECTS_DIR}/slack-trading/src/cmd/stats/src
```

Note that in order to run the twirpy plugin, `cmd/backtester/venv/bin` must be in the terminal's PATH.

## Debugging
Port forward to the production ESDB instance:
``` bash
kubectl port-forward svc/eventstoredb 21133:2113 -n eventstoredb
```

## Profiling
Pprof can be easily set up to do profiling:
``` bash
go tool pprof -seconds 30 -http localhost:8090 myserver http://localhost:8080/debug/pprof/profile
```

## Container Registry
Docker containers are hosted on vultr. Before pushing and pulling, you need to login.
``` bash
docker login https://ewr.vultrcr.com/grodt -u $VULTR_REGISTRY_USER -p $VULTR_REGISTRY_PASS
```
`VULTR_REGISTRY_USER` and `VULTR_REGISTRY_PASS` can be found on the Vultr console.

## Installing bump2version
``` bash
python3 -m ensurepip --upgrade
python3 -m pip install --user bump2version
```

### Managing
1. Each Dockerfile has a `# Version: 1.x.x` at the top.
2. Each Dockerfile also has a `.bumpversion.cfg` file, since we want to manage each version number separately.

### Deploying a new version
The following script takes care of updating the version of the Dockerfile and deploying it to the container registry, updating the Kubernetes deployment file, and pushing the code to Github.
``` bash
./deploy-app.sh <version>
```
Version can be: patch, minor, major
---
Similarly, the base images can be deploy (if necessary) with the following commands:
``` bash
./deploy-base-image.sh <version>
./deploy-base-image-2.sh <version>
```

# Deployment
Our production environment is hosted on vultr and managed with fluxcd. Manifests are stored in `.clusters/production`

## Spin up a New Cluster
To start a new cluster, navigate to the Vultr dashboard, click "Kubernetes" and "+ Add Cluster."

### Install the SealedSecrets controller
``` bash
brew install helm
helm repo add bitnami https://charts.bitnami.com/bitnami
kubectl create namespace sealed-secrets
helm install sealed-secrets bitnami/sealed-secrets --namespace sealed-secrets
```

### Install postgres
Postgres is used to store trade data.

#### Secrets
The postgres secret file is not checked into version control. Add the following file to `${PROJECTS_DIR}/slack-trading/.clusters/production/postgres-secret.yaml`:
``` yaml
apiVersion: v1
kind: Secret
metadata:
  name: postgres-secret
  namespace: database
type: Opaque
data:
  POSTGRES_PASSWORD: $(echo -n "yourpassword" | base64)
```

A new cluster can be spun up in kubernetes with the following commands:
``` bash
kubectl create namespace database
kubectl apply -f ${PROJECTS_DIR}/slack-trading/.clusters/production/postgres-configmap.yaml
kubectl apply -f ${PROJECTS_DIR}/slack-trading/.clusters/production/postgres-secret.yaml
kubectl apply -f ${PROJECTS_DIR}/slack-trading/.clusters/production/postgres-pvc.yaml
kubectl apply -f ${PROJECTS_DIR}/slack-trading/.clusters/production/postgres-service.yaml
kubectl apply -f ${PROJECTS_DIR}/slack-trading/.clusters/production/postgres-deployment.yaml
```

### Development
In order to use the app locally, you will need to port-forward the connection:
``` bash
kubectl port-forward svc/postgres 5432:5432 -n database
```

## Create Playground Database
In a sql editor, run:
``` bash
CREATE database playground;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE SEQUENCE IF NOT EXISTS order_id_seq START 1;
```

### Add a Deploy Key to the Cluster (this can be skipped if the sealedsecret has already been created)
Flux needs a deploy key in order to pull from GitHub. Create one and then apply it to the cluster as a sealed secret.

First, create a namespace for the app and the database
``` bash
kubectl create namespace eventstoredb
```

Second, create a normal secret from the private key file
``` bash
kubectl create secret generic flux-git-deploy \
  --namespace default \
  --from-file=identity=<absolute-path-to-your-private-key> \
  --dry-run=client -o yaml > secret.yaml
```

Third, convert the secret into a sealed secret:
``` bash
kubeseal  --controller-name=sealed-secrets --controller-namespace=sealed-secrets --format yaml < secret.yaml > ${PROJECTS_DIR}/slack-trading/.clusters/production/sealedsecret-flux-git-deploy.yaml
```

Fourth, apply the sealed secret to the cluster
``` bash
kubectl apply -f ${PROJECTS_DIR}/slack-trading/.clusters/production/sealedsecret-flux-git-deploy.yaml
```

### Bootstrap the Cluster
You will need a personal access token in order to bootstrap the cluster.

#### Getting a GitHub personal access token
1. Go to GitHub:

2. Navigate to GitHub and sign in to your account.
Go to Settings:

3. In the top-right corner of any GitHub page, click on your profile picture, then click on Settings.
Access Developer Settings:

4. In the left sidebar, scroll down and click on Developer settings.
Generate a New Personal Access Token:

In the Developer settings page, click on Personal access tokens.
Then, click on Tokens (classic), and click on Generate new token or Generate new token (classic).
Select Scopes:

Give the token a descriptive name (e.g., "Flux GitOps token").

Set an expiration date for the token (or leave it with no expiration if necessary, though it's recommended to have an expiration).

For Flux, you typically need the following scopes:

- repo: Full control of private repositories (if you need to deploy from private repositories).
- workflow: Update GitHub Actions workflows (optional, if you use GitHub Actions).
- write:packages: Push packages to GitHub packages (optional, if you are working with GitHub packages).
- admin:repo_hook: Manage webhooks (optional, needed if Flux will create webhooks).

#### Run the command
``` bash
flux bootstrap github --owner=jiaming2012 --repository=slack-trading --branch=main --path=.clusters/production --personal
```

### Generate signals
The heart of the program grabs tick data from polygon and generates signals from them

``` bash
cd ${PROJECTS_DIR}/slack-trading/src/cmd/stats
source env/bin/activate
python generate_signals.py
```

## Connect to an Existing Cluster
### Configure Local Machine to Remote Cluster
Log into the Vultr dashboard and download the cluster's config file.
``` bash
export KUBECONFIG=export KUBECONFIG="/Users/jamal/projects/grodt/vultr-k8s.yaml"
```

### Secrets
We use `kubeseal` for managing encrypted secrets
``` bash
brew install kubeseal
```

Assuming you have `.env.production` in the `src/` directory, convert an env file to a kubernetes secret:
``` bash
cd path/to/cmd
./convert_env_to_secret.sh
```

Convert the secret to a sealed secret and deploy with the sealed secret with:
``` bash
kubeseal --controller-name=sealed-secrets --controller-namespace=sealed-secrets --format yaml < secret.yaml > .clusters/production/sealedsecret.yaml
```

### Config
Similarly, create a configmap from an env file with command:
``` bash
cd path/to/cmd
./convert_env_to_config.sh
```

Remove any variables that you do not wish to use.

### Connecting
Go to the vultr dashboard and download the kube context file.
``` bash
export KUBECONFIG=/Users/jamal/projects/grodt/vultr-k8s.yaml
```

# Data
In order to run scripts for importing data into eventstore db:
``` bash
kubectl port-forward pod/eventstoredb-0 2113:2113 -n eventstoredb
```
You can now run import scripts from local machine.

# Telemetry
Currently using the free tier of telemetry cloud: https://grafana.com/orgs/jac475. Used the following guide to set up: https://grafana.com/docs/grafana-cloud/monitor-applications/application-observability/setup/quickstart/go/

## See Telemetry Data
1. Launch Grafana
2. Click "Data Sources" in the side menu
3. Click grafanacloud-jac475-traces -> "Explore"
4. Select: Query type -> "Search"

# Running locally
## Install golang and python
Both golang:1.20 and python3.10 are required.

If installing on ubuntu:
``` bash
sudo apt-get install python3.10-venv
```

## Set your PythonPath
``` bash
export PYTHONPATH=${PROJECTS_DIR}/slack-trading:${PROJECTS_DIR}/slack-trading/src/cmd/stats:${PYTHONPATH}
```

## Initiate your python env
``` bash
task python:install
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