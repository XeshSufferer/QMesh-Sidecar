#!/bin/bash
set -e

PROXY_PORT="${PROXY_PORT:-3128}"
GOSSIP_PORT="${GOSSIP_PORT:-4221}"
QUIC_PORT="${QUIC_PORT:-4224}"

echo "Setting up transparent proxy with iptables + TPROXY"
echo "Proxy port: ${PROXY_PORT}"

# Enable IP forwarding
echo 1 > /proc/sys/net/ipv4/ip_forward
echo 0 > /proc/sys/net/ipv4/conf/all/rp_filter

# Create a custom chain for QMesh
iptables -t mangle -N QMESH_PROXY 2>/dev/null || true
iptables -t mangle -F QMESH_PROXY

# Mark packets that should be proxied (destination port 80, 443, or custom HTTP ports)
# Exclude traffic to localhost and pod's own IP
iptables -t mangle -A QMESH_PROXY -d 127.0.0.1/8 -j RETURN
iptables -t mangle -A QMESH_PROXY -d ${POD_IP:-0.0.0.0}/32 -j RETURN

# Redirect HTTP traffic (port 80) to our proxy
iptables -t mangle -A QMESH_PROXY -p tcp --dport 80 -j TPROXY --on-port ${PROXY_PORT} --tproxy-mark 0x1/0x1

# Redirect HTTPS traffic (port 443) - needs SSL bumping, but for now just log
iptables -t mangle -A QMESH_PROXY -p tcp --dport 443 -j TPROXY --on-port ${PROXY_PORT} --tproxy-mark 0x1/0x1

# Apply the chain to OUTPUT chain (for traffic from this pod)
iptables -t mangle -A OUTPUT -j QMESH_PROXY

# Add policy routing for marked packets
ip rule add fwmark 0x1 lookup 100 2>/dev/null || true
ip route add local 0.0.0.0/0 dev lo table 100 2>/dev/null || true

echo "iptables rules applied successfully"
iptables -t mangle -L -n -v

echo "Transparent proxy setup complete"
