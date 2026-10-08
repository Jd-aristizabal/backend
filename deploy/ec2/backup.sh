#!/bin/sh
set -eu
cd /opt/johan-backend
umask 077
mkdir -p backups
docker compose exec -T postgres pg_dump -U portfolio_admin -d portfolio -Fc > "backups/portfolio-$(date -u +%Y%m%dT%H%M%SZ).dump"
find backups -type f -name 'portfolio-*.dump' -mtime +7 -delete
