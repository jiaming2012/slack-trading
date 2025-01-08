# Backtestester API
Provides a unified model to **train models** -> **backtest models** -> **live trading**

# Installations
There are multiple python clients, each which will spin up a live playground. All python clients will run on a linux VM

## Steps
1. Spin a VPS with similar specs -- put the VPS in the same private network as the K8s cluster
- vCPU/s:
2 vCPUs
- RAM:
2048.00 MB
- Storage:
80 GB NVMe 

2. Install python
``` bash
sudo apt-get update
sudo apt-get install -y libssl-dev openssl
wget https://www.python.org/ftp/python/3.10.0/Python-3.10.0.tgz && \
    tar xvf Python-3.10.0.tgz && \
    cd Python-3.10.0 && \
    ./configure --enable-optimizations && \
    make altinstall
```
3. Install node
``` bash
curl -sL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs
```
4. Install dependencies
``` bash
sudo npm install pm2 -g
```

5. Download Source

<b>i</b>. Either create new deploy keys and add them to the `slack-trading` repo, 
``` bash
ssh-keygen
```
<b>ii</b>. or copy the already created keys to the deploy machines `~/.ssh` folder. On the deployment machine:

On local machine run:
``` bash
VULTR_IP=""
scp ${PROJECTS_DIR}/slack-trading/vultr_ml_id_rsa root@${VULTR_IP}:/root/.ssh/id_rsa
```
On remote machine run:
``` bash
git clone git@github.com:jiaming2012/slack-trading.git
```

6. Install the Source
``` bash
cd /root/slack-trading/src/cmd/stats
python3.10 -m venv venv
source venv/bin/activate
pip install --upgrade pip setuptools wheel
pip install -r requirements.txt
```


# Architecture
`Playground.Tick()` will automatically increment to the next tick before applying any processing. Hence in the following example, the zero value of the tick data feed will be ignored, and the first tick processed will be `100.0`:

``` go
prices := []float64{0, 100.0, 115.0}
feed := mock.NewMockBacktesterDataFeed()
```

## Protobufs
Protobufs are used to speed up communication with API clients.

## Compiling
``` bash
cd ${PROJECTS_DIR}/slack-trading/src/backtester-api
protoc --go_out=./playground --go_opt=paths=source_relative --go-grpc_out=./playground --go-grpc_opt=paths=source_relative playground.proto
```