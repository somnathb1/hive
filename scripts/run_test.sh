#!/bin/bash
# set -x
# Default values
TEST_NAME="/Blob Transaction Ordering, Single Account, Dual Blob"
EXPERIMENT="debug"
CLIENT="nethermind-gnosis"
SIMULATOR="ethereum/engine"
SIMULATOR_LOG_LEVEL=3
CLIENT_LOG_LEVEL=3
PROXY=""
MITMPROXY_ADDITIONAL_ARGS='-s "/home/mitmproxy/.mitmproxy/stop_test.py"'

while [[ $# -gt 0 ]]; do
  case $1 in
    -t|--test)
      TEST_NAME="$2"
      shift # past argument
      shift # past value
      ;;
    -e|--exp)
      EXPERIMENT="$2"
      shift
      shift
      ;;
    -c|--client)
      CLIENT="$2"
      shift
      shift
      ;;
    -s|--simulator)
      SIMULATOR="$2"
      shift
      shift
      ;;
    -p|--proxy)
      PROXY="$2"
      shift
      shift
      ;;
    *)    # unknown option
      shift # past argument
      ;;
  esac
done

echo "Started test: '$TEST_NAME' and experiment: '$EXPERIMENT' with client: '$CLIENT' and simulator: '$SIMULATOR'"

echo "Starting docker server on port 9090"
go run scripts/server.go -port 9090 &

if [ "$PROXY" == "" ]; then
  echo "Proxy not set"
else
  echo "Using proxy: $PROXY"
  # Get mitmproxy container ID
  CONTAINER_ID=$(docker run -d -e MITMPROXY_EXPERIMENT_ID=$EXPERIMENT -p 7080:8080 -p 8089:8089 -p 8082:8082 -v $PWD/scripts/proxy:/home/mitmproxy/.mitmproxy mitmproxy/mitmproxy mitmweb --listen-host 0.0.0.0 --web-host 0.0.0.0 --listen-port 8089 --web-port 8082 --set ssl_insecure=true $MITMPROXY_ADDITIONAL_ARGS --set hardump="/home/mitmproxy/.mitmproxy/$EXPERIMENT.har")
fi

HTTP_PROXY="$PROXY" ./hive --sim "$SIMULATOR" --sim.limit "$TEST_NAME" --client "$CLIENT" --loglevel=$CLIENT_LOG_LEVEL --sim.loglevel=$SIMULATOR_LOG_LEVEL --docker.output --results-root="scripts/experiments/$EXPERIMENT/runs" --dev.addr=127.0.0.1:3000

if [ "$PROXY" == "" ]; then
  echo "No need to stop proxy"
else
  docker logs $CONTAINER_ID
  docker stop "$CONTAINER_ID"
  mv "scripts/proxy/$EXPERIMENT.har" "scripts/experiments/$EXPERIMENT"
fi

echo "Stopping docker server"
curl -X POST "http://localhost:9090/stop"

echo "Finished test: '$TEST_NAME' and experiment: '$EXPERIMENT'"
