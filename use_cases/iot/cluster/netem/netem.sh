#!/bin/sh
# Adds simulated WAN latency to this node's traffic toward nodes in other regions.
# Runs inside a node's network namespace. Usage: netem.sh on|off|status (needs MY_REGION).
set -eu
DEV=eth0
MODE="${1:-on}"

# One-way delay in ms: half of the typical AWS round trip between the regions this demo stands in for.
#   region_1 = us-east-1, region_2 = eu-central-1, region_3 = ap-southeast-1
delay_ms() {
  case "$1-$2" in
    region_1-region_2|region_2-region_1) echo 45 ;;
    region_2-region_3|region_3-region_2) echo 78 ;;
    region_1-region_3|region_3-region_1) echo 110 ;;
  esac
}

nodes_in() {
  case "$1" in
    region_1) echo "lat-roach1 lat-roach2 lat-roach3" ;;
    region_2) echo "lat-roach4 lat-roach5 lat-roach6" ;;
    region_3) echo "lat-roach7 lat-roach8 lat-roach9" ;;
  esac
}

# Stopped nodes don't resolve; skip them (latency is re-applied to every running node when one starts).
resolve() {
  for _ in 1 2 3; do
    ip=$(getent hosts "$1" | awk '{print $1}' | head -1)
    [ -n "$ip" ] && { echo "$ip"; return; }
    sleep 1
  done
}

case "$MODE" in
  off)
    tc qdisc del dev "$DEV" root 2>/dev/null || true
    echo "$MY_REGION: latency off"
    ;;
  status)
    tc qdisc show dev "$DEV"
    ;;
  on)
    tc qdisc del dev "$DEV" root 2>/dev/null || true
    # Band 1 (default) is undelayed; bands 2 and 3 carry traffic to the two other regions.
    tc qdisc add dev "$DEV" root handle 1: prio bands 4 priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
    band=2
    for region in region_1 region_2 region_3; do
      [ "$region" = "$MY_REGION" ] && continue
      d=$(delay_ms "$MY_REGION" "$region")
      tc qdisc add dev "$DEV" parent "1:$band" handle "${band}0:" netem delay "${d}ms" limit 100000
      for node in $(nodes_in "$region"); do
        ip=$(resolve "$node")
        [ -z "$ip" ] && { echo "$MY_REGION: $node is down, skipped"; continue; }
        tc filter add dev "$DEV" parent 1: protocol ip prio 1 u32 match ip dst "$ip/32" flowid "1:$band"
      done
      echo "$MY_REGION -> $region: +${d}ms one way"
      band=$((band + 1))
    done
    ;;
  *)
    echo "usage: netem.sh on|off|status" >&2
    exit 1
    ;;
esac
