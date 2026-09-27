#!/bin/bash
# Installs Caddy as a host-network container on the instance and points it at the
# Caddyfile. Idempotent: re-running replaces the container and reloads config.
#
# Run this only once DNS for both names resolves to this instance. Caddy requests
# certificates on first request, and Let's Encrypt rate-limits failed validations
# (5 per hostname per hour), so starting it early costs you retries.
set -euo pipefail

CADDYFILE=${1:?usage: install.sh /path/to/Caddyfile}

install -d -m 755 /etc/caddy
install -m 644 "$CADDYFILE" /etc/caddy/Caddyfile

docker stop caddy 2>/dev/null || true
docker rm caddy 2>/dev/null || true

# --network host: the backends listen on loopback, and Caddy needs 80/443 on the
# host anyway. A bridge network would make 127.0.0.1 mean the Caddy container.
docker run -d \
  --name caddy \
  --restart=always \
  --network host \
  -v /etc/caddy/Caddyfile:/etc/caddy/Caddyfile:ro \
  -v caddy_data:/data \
  -v caddy_config:/config \
  caddy:2-alpine

echo "waiting for Caddy to answer on :80"
for i in $(seq 1 12); do
  if curl -fsS -o /dev/null http://127.0.0.1:80 2>/dev/null; then
    echo "caddy up"
    exit 0
  fi
  sleep 5
done
echo "caddy did not answer on :80; logs:"
docker logs --tail 40 caddy
exit 1
