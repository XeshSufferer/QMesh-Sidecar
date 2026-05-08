#!/bin/bash
set -e

NAMESPACE="${NAMESPACE:-default}"
SERVICE_NAME="${SERVICE_NAME:-qmesh-sidecar}"
GOSSIP_PORT="${GOSSIP_PORT:-4221}"
OUTPUT_FILE="${OUTPUT_FILE:-/shared/gossip-seeds}"

echo "QMesh Sidecar Seed Discovery"
echo "Namespace: ${NAMESPACE}"
echo "Service: ${SERVICE_NAME}"

# Wait for Kubernetes API
sleep 2

# Get endpoints from headless service
SEEDS=""
if command -v kubectl &> /dev/null; then
    echo "Using kubectl to discover endpoints..."
    ENDPOINTS=$(kubectl get endpoints "${SERVICE_NAME}" -n "${NAMESPACE}" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || echo "")
    
    if [ -n "$ENDPOINTS" ]; then
        for ip in $ENDPOINTS; do
            if [ -n "$SEEDS" ]; then
                SEEDS="${SEEDS},${ip}:${GOSSIP_PORT}"
            else
                SEEDS="${ip}:${GOSSIP_PORT}"
            fi
        done
    fi
else
    echo "kubectl not found, using DNS resolution..."
    # Fallback to DNS resolution
    DNS_RESULT=$(nslookup "${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local" 2>/dev/null | grep "Address" | tail -n +2 | awk '{print $2}' | sort -u || echo "")
    
    for ip in $DNS_RESULT; do
        if [ -n "$SEEDS" ]; then
            SEEDS="${SEEDS},${ip}:${GOSSIP_PORT}"
        else
            SEEDS="${ip}:${GOSSIP_PORT}"
        fi
    done
fi

# Remove own IP if POD_IP is set
if [ -n "${POD_IP}" ] && [ -n "$SEEDS" ]; then
    SEEDS=$(echo "$SEEDS" | sed "s/${POD_IP}:${GOSSIP_PORT}//g" | sed 's/,,/,/g' | sed 's/^,//' | sed 's/,$//')
fi

echo "Discovered seeds: ${SEEDS:-none}"

# Write to shared volume
mkdir -p "$(dirname "${OUTPUT_FILE}")"
echo "$SEEDS" > "$OUTPUT_FILE"

echo "Seeds written to ${OUTPUT_FILE}"
cat "$OUTPUT_FILE"
