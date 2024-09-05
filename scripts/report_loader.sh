#!/bin/bash
# Default values
PORT=8090
PROXY_PORT=8089
EXPERIMENT="01-nethermind-withdrawals-jq"
HIVEVIEW_PORT=3001
PROXY_CONTAINER_NAME="recovery_proxy"

while [[ $# -gt 0 ]]; do
  case $1 in
    -e|--exp)
      EXPERIMENT="$2"
      shift # past argument
      shift # past value
      ;;
    -p|--port)
      PORT="$2"
      shift # past argument
      shift # past value
      ;;
    -hp|--hive_port)
      HIVEVIEW_PORT="$2"
      shift # past argument
      shift # past value
      ;;
    -pn|--proxy_container_name)
      PROXY_CONTAINER_NAME="$2"
      shift # past argument
      shift # past value
      ;;
    *)    # unknown option
      shift # past argument
      ;;
  esac
done

echo "Recovery experiment: '$EXPERIMENT' and open mitmproxy on port '$PORT'"

cp "scripts/experiments/$EXPERIMENT/$EXPERIMENT.har" "$PWD/scripts/proxy"
docker stop $PROXY_CONTAINER_NAME || echo "Container $PROXY_CONTAINER_NAME does not started"
docker rm $PROXY_CONTAINER_NAME || echo "Container recovery_proxy does not exist"
echo "Started mitmproxy on port 8090. To stop it run 'docker stop recovery_proxy'"
docker run --name $PROXY_CONTAINER_NAME -d -p $PORT:8082 -p $PROXY_PORT:8089 -v $PWD/scripts/proxy:/home/mitmproxy/.mitmproxy mitmproxy/mitmproxy mitmweb --listen-host 0.0.0.0 --web-host 0.0.0.0 --listen-port 8089 --web-port 8082 --set ssl_insecure=true -r "/home/mitmproxy/.mitmproxy/$EXPERIMENT.har"

PID=$(lsof -ti :$HIVEVIEW_PORT)

if [ ! -z "$PID" ]; then
  kill $PID
  echo "Hiveview on port $HIVEVIEW_PORT killed."
else
  echo "No process found on port $HIVEVIEW_PORT."
fi
echo "Started hiveview on port $HIVEVIEW_PORT for experiment $EXPERIMENT"
./hiveview --serve --logdir "scripts/experiments/$EXPERIMENT/runs" -addr 0.0.0.0:$HIVEVIEW_PORT &
echo "To stop hiveview run 'kill $(lsof -ti :$HIVEVIEW_PORT)'"
echo "Done"