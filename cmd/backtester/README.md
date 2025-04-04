Backtester is based off of https://pypi.org/project/Backtesting/

# Deploy to the Cloud

## Install Cert Manager
``` bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --version v1.5.3 --set installCRDs=true
```

## Port Forward
``` bash
export VULTR_IP=""
scp vke.yaml root@${VULTR_IP}:/root
```

SSH onto the remote machine:
``` bash
ssh root@${VULTR_IP}
snap install kubectl --classic
tmux new -s port-forward
export KUBECONFIG="/root/vke.yaml"
kubectl port-forward svc/grodt-lb 50051:50051
```
Detach from the tmux session by pressing Ctrl+B followed by D.

To reattach to the session: 
``` bash
tmux attach -t port-forward
```

### Postrgres
In order to connect to postgres for development
``` bash
kubectl port-forward svc/postgres 5432:5432 -n database
```

## App
On the remote terminal:
``` bash
tmux new -s app
export PROJECTS_DIR="/root"
cd ${PROJECTS_DIR}/slack-trading/cmd/backtester
git checkout dev
source venv/bin/activate
python proximal_policy_optimization_v11.py --host localhost:50051
```

## Instructions
1. Start a vultr instance. Select SSH keys
2. SSH into instance
``` bash
ssh root@ip_address
```
ip_address can be found in the Vultr console.

![Vultr Console](<Screenshot 2024-11-04 at 10.00.35 AM.png>)

## Download Source
Either create new deploy keys and add them to the `slack-trading` repo, 
``` bash
ssh-keygen
```
or copy the already created keys to the deploy machines `~/.ssh` folder. On the deployment machine:
``` bash
VULTR_IP=""
scp ${PROJECTS_DIR}/slack-trading/vultr_ml_id_rsa root@${VULTR_IP}:/root/.ssh/id_rsa
```
### Pull the Source
``` bash
git clone git@github.com:jiaming2012/slack-trading.git
```

### Managing the server
Mosh allow better connecting handling to the server, improving reconnects.
``` bash
sudo apt-get install -y mosh
sudo ufw allow 60000:61000/udp
sudo ufw reload
```

Mosh also needs to be installed on your local machine:
``` bash
brew install mosh
```

### Build the Source
``` bash
add-apt-repository ppa:deadsnakes/ppa
apt update
apt install -y python3.10 python3.10-venv python3.10-dev
cd /root/slack-trading/src/cmd/stats
python3.10 -m venv env
source venv/bin/activate
pip install --upgrade pip setuptools wheel
pip install -r requirements.txt
sudo snap install task --classic
```

### Install pm2: Production Process Manager
``` bash
curl -sL https://deb.nodesource.com/setup_18.x | sudo -E bash
sudo apt-get install -y nodejs
sudo npm install pm2 -g
pm2 completion install
```

## Set environment variables
``` bash
nano ~/.bashrc
```

### Add the following lines
``` bash
export PROJECTS_DIR="/root"
export PYTHONPATH=${PROJECTS_DIR}/slack-trading:${PROJECTS_DIR}/slack-trading/src/cmd/stats:${PYTHONPATH}
```

#### In Background
To run in the background:
``` bash
tmux
task optimize:daily
```
Detach the session with Ctrl+B followed by D

To reattach to the session:
``` bash
tmux attach-session -t 0
```

To list all `tmux` sessions:
``` bash
tmux ls
```

#### In Foreground
``` bash
/root/slack-trading/cmd/backtester/venv/bin/python /root/slack-trading/cmd/backtester/proximal_policy_optimization_v3_5.py
```

# Installation
``` bash
cd ${PROJECTS_DIR}/slack-trading/cmd/backtester
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

# Proximal Policy Optimization
Runs a machine learning reinforcement algorithm to train a model on a particular strategy, leveraging the backtester playground API.

# Prepare the Data
First, we want to generate fake training data
``` bash
cd ${PROJECTS_DIR}/slack-trading/src/cmd/stats
go run generate_data/main.go
mv stock_data.csv ${PROJECTS_DIR}/slack-trading/src/backtester-api/data/training_data.csv
```

Next, repeat the process to generate validation data
``` bash
go run generate_data/main.go
mv stock_data.csv ${PROJECTS_DIR}/slack-trading/src/backtester-api/data/validation_data.csv
```

## Plot the Data
``` bash
python ${PROJECTS_DIR}/slack-trading/src/cmd/stats/plot_candlestick.py ${PROJECTS_DIR}/slack-trading/src/backtester-api/data/training_data.csv
```

## Run Event Main
Event main is needed to use the playground api.
``` bash
cd ${PROJECTS_DIR}/slack-trading/src/eventmain
./run-dev.sh
```

## Run the PPO
First we have to install of stats python packages:
``` bash
cd ${PROJECTS_DIR}/slack-trading/src/cmd/stats
source env/bin/activate
pip install -r requirements.txt
```

Finally we can run our script:
``` bash
cd ${PROJECTS_DIR}/slack-trading
./cmd/backtester/venv/bin/python cmd/backtester/proximal_policy_optimization.py
```