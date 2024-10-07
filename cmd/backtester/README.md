Backtester is based off of https://pypi.org/project/Backtesting/

# Installation
``` bash
cd ${PROJECTS_DIR}/slack-trading/cmd/backtester
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

# Proximal Policy Optimization
Runs a machine learning reinforcement algorithm to train a model on a particular strategy, leveraging the backtester playground API.

# Run
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

## Plot the data
``` bash
python ${PROJECTS_DIR}/slack-trading/src/cmd/stats/plot_candlestick.py ${PROJECTS_DIR}/slack-trading/src/backtester-api/data/training_data.csv
```