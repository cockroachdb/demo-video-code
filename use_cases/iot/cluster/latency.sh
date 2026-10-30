#!/usr/bin/env bash
# Toggle the simulated WAN latency on the docker-compose.latency.yml cluster without restarting it.
#   ./latency.sh on | off | status
set -euo pipefail
cd "$(dirname "$0")"
MODE="${1:-status}"

for n in 1 2 3 4 5 6 7 8 9; do
  region="region_$(( (n - 1) / 3 + 1 ))"
  docker run --rm --net "container:lat-roach$n" --cap-add NET_ADMIN -e "MY_REGION=$region" localhost/lat-netem "$MODE" \
    | sed "s/^/lat-roach$n: /"
done
