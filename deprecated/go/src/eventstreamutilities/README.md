# Run
To run in development environment:
``` bash
./run-dev.sh
```

To run production environment:
``` bash
./run-prod.sh
```

## With command line args
In order to avoid passing arguments via standard input, you can pass environment variables. For example to run the fetch tradier options command for Nvidia, run:
``` bash
./run-prod.sh FETCH_AND_STORE_TRADIER_OPTION_CONTRACTS nvda
```