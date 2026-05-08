#!/bin/sh
set -e

SERVICE_NAME="${SERVICE_NAME:-qmesh-sidecar}"
NAMESPACE="${NAMESPACE:-default}"
GOSSIP_PORT="${GOSSIP_PORT:-4221}"

echo "Discovering QMesh seeds from headless service: ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local"

# Wait for DNS to be ready
sleep 2

# Resolve all A records for the headless service
# Format returned by nslookup: multiple Address entries
SEEDS=""
ADDRESSES=$(nslookup "${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local" 2>/dev/null | grep -A 100 "Address" | grep -E "^Address: " | awk '{print $2}' | sort -u)

for addr in $ADDRESSES; do
    if [ -n "$SEEDS" ]; then
        SEEDS="${SEEDS},${addr}:${GOSSIP_PORT}"
    else
        SEEDS="${addr}:${GOSSIP_PORT}"
    fi
done

if [ -z "$SEEDS" ]; then
    echo "No seeds discovered, starting as seed node"
    export GOSSIP_SEEDS=""
else
    echo "Discovered seeds: ${SEEDS}"
    export GOSSIP_SEEDS="$SEEDS"
fi

# Write seeds to shared volume for sidecar to read
echo "$GOSSIP_SEEDS" > /shared/gossip-seeds
echo "Seeds written to /shared/gossip-seeds"
