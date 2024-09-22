
Currently testing a MVP, using Clickhouse as a tick database.

# Installation
``` bash
docker pull clickhouse/clickhouse-server
docker run -d --name clickhouse-server --privileged -p 8123:8123 -p 9000:9000 -p 9009:9009 clickhouse/clickhouse-server```
```

## Install the ClickHouse Client
If installing on mac:
``` bash
brew install clickhouse
```

Log into clickhouse client:
``` bash
clickhouse client
```

## Check Installation is Running
``` bash
docker logs clickhouse-server
curl 'http://localhost:8123/?query=SHOW%20DATABASES'
```