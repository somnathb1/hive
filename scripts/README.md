# Scripts Module

This module contains various scripts to assist in the development of the project.

## run_tests.sh

This script allows us to run tests and store all logs, JRPC requests, and responses in a single location (the experiments directory). The script can:

1.	Run tests against any combination of clients and simulators.
2.	Store all logs, JRPC requests, and responses in one place (the experiments directory).
3.	Run tests with a proxy to capture all JRPC requests and responses.
4.	Copy the genesis file and other necessary files from the running client to a specified directory.

Usage:
```bash
./scripts/run_test.sh --test "/Blob Transaction Ordering, Single Account, Dual Blob" --exp "01" --client "nethermind-gnosis" --simulator "ethereum/engine" --proxy "192.168.3.49:8089" > ./scripts/test.log
```

### Options:
-t|--test - test regular expression to run, e.g. `/Blob Transaction Ordering, Single Account, Dual Blob
-e|--exp - experiment number, directory inside scripts/experiments, e.g. `01`
-c|--client - client name, e.g. `nethermind-gnosis`
-s|--simulator - simulator name, e.g. `ethereum/engine`
-p|--proxy - IP address of the local machine and mitmproxy port, e.g. `192.168.3.49:8089`

## Using mitmproxy
To use mitmproxy, we need to run it on the local machine and set the proxy URL (`HTTP_PROXY` environment variable).
For example, `./scripts/run_test.sh -p 192.168.3.49:8089`.

The goal of the proxy is to capture all requests and responses between the client and the simulator. 
Additionally, it is possible to use scripts to gather data, pause, or modify requests and responses.

### Existing mitmproxy scripts
#### stop_test.py

This script pauses the test execution after a specific condition is met. All configuration details are described in the scripts/proxy/config.ini file. Options:

•	target_url - Docker server URL (`server.go`) that allows copying files between the container and the host machine.
•	copy_files - If set to `True`, the script will copy files from the container to the host machine (needs also to specify `test_end`).
•	test_end - The endpoint of requests after which mitmproxy will pause the script, e.g., `/testsuite/1/test/3`.
•	end_wait_time - The time in seconds the script will wait after resuming the test, e.g., `10`.

[copy_files] - This INI section contains files that should be copied from the container. The section consists of the following fields:

•	container_file_path_1 = /genesis.json
•	host_file_path_1 = scripts/experiments

To add additional files, use the same pattern, e.g.,

•	container_file_path_2 = /mydir/otherfile.json
•	host_file_path_2 = scripts/experiments

## server.go
The app is a simple HTTP server that allows to copy files and directories between the container and the host machine.
To run the server, use the following command:
```bash
go run scripts/server.go -port 9090
```

Usage:
```bash
curl -X POST -H "Content-Type: application/json" -d '{            
  "container_id": "d136581a900e",
  "container_file_path": "/genesis.json",
  "host_file_path": "scripts/experiments/01/genesis.json"
}' "http://localhost:9090/copyfile"
```
Will copy `/genesis.json` from the container with the id `d136581a900e` to the host machine to the `scripts/experiments/01/genesis.json`.

```bash
curl -X POST "http://localhost:8080/stop"
```
Will stop the server.

## clear_debug.sh
Clear `scripts/experiments/01` directory and move all files to `scripts/experiments/01-prev`.

## report_loader.sh
Load the archive of requests and responses from the `scripts/experiments/01` to the new mitmproxy instance. Load hive report.
