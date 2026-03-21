#!/bin/bash
# Run once on a fresh Ubuntu 22.04 EC2 instance.
# Usage: chmod +x setup-ec2.sh && sudo ./setup-ec2.sh

set -euo pipefail

echo "==> Updating packages"
apt-get update -q && apt-get upgrade -yq

echo "==> Installing Docker"
apt-get install -yq ca-certificates curl gnupg
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  > /etc/apt/sources.list.d/docker.list
apt-get update -q
apt-get install -yq docker-ce docker-ce-cli containerd.io docker-compose-plugin
usermod -aG docker ubuntu

echo "==> Installing nginx"
apt-get install -yq nginx
systemctl enable nginx

echo "==> Copying nginx config"
cp /home/ubuntu/infra/nginx.conf /etc/nginx/sites-available/webhookdb
ln -sf /etc/nginx/sites-available/webhookdb /etc/nginx/sites-enabled/webhookdb
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx

echo "==> Done. Next steps:"
echo "  1. cd /home/ubuntu/backend"
echo "  2. cp .env.production.example .env.production && nano .env.production"
echo "  3. docker compose -f docker-compose.prod.yml up -d --build"
