Backtester is based off of https://pypi.org/project/Backtesting/

# Deploy to the Cloud
1. Start a vultr instance. Select SSH keys
2. SSH into instance
``` bash
ssh root@ip_address
```
ip_address can be found in the Vultr console.

## Download Source
Either create new deploy keys and add them to the `slack-trading` repo, 
``` bash
ssh-keygen
```
or copy the already created keys to the deploy machines `~/.ssh` folder. On the deployment machine:
``` bash
VULTR_IP=""
scp vultr_ml_id_rsa root@${VULTR_IP}:/root/.ssh/id_rsa
```
### Pull the Source
``` bash
git clone git@github.com:jiaming2012/slack-trading.git
```

### Build the Source
``` bash
add-apt-repository ppa:deadsnakes/ppa
apt update
apt install -y python3.10 python3.10-venv python3.10-dev
cd /root/slack-trading/cmd/backtester
python3.10 -m venv venv
source venv/bin/activate
pip install --upgrade pip setuptools wheel
pip install -r requirements.txt
```

### Run the App
``` bash
export PROJECTS_DIR="/root"
```

#### In Background
To run in the background:
``` bash
tmux
/root/slack-trading/cmd/backtester/venv/bin/python /root/slack-trading/cmd/backtester/proximal_policy_optimization_v3_5.py
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