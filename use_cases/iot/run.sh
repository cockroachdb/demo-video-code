#!/usr/bin/env bash
# Start/stop the three regional app instances against the docker-compose.latency.yml cluster.
#   iot/run.sh start | stop | restart
# Each instance prefers its own region's gateway and fails over to the next-nearest region.
set -euo pipefail
cd "$(dirname "$0")/.."

US=localhost:26257 EU=localhost:26258 AP=localhost:26259
url() { echo "postgres://root@$1,$2,$3/iot?sslmode=disable&connect_timeout=2&pool_max_conn_lifetime=20s"; }

start() {
  go build -o /tmp/iot-app ./iot/app
  DATABASE_URL="$(url $US $EU $AP)" REGION=region_1 LISTEN_ADDR=127.0.0.1:3000 nohup /tmp/iot-app > /tmp/iot-region_1.log 2>&1 &
  DATABASE_URL="$(url $EU $US $AP)" REGION=region_2 LISTEN_ADDR=127.0.0.1:3001 nohup /tmp/iot-app > /tmp/iot-region_2.log 2>&1 &
  DATABASE_URL="$(url $AP $EU $US)" REGION=region_3 LISTEN_ADDR=127.0.0.1:3002 nohup /tmp/iot-app > /tmp/iot-region_3.log 2>&1 &
  echo "started: map API on http://localhost:3000, logs in /tmp/iot-region_*.log"
}

stop() { pkill -f /tmp/iot-app || true; }

case "${1:-start}" in
  start) start ;;
  stop) stop ;;
  restart) stop; sleep 1; start ;;
  *) echo "usage: iot/run.sh start|stop|restart" >&2; exit 1 ;;
esac
