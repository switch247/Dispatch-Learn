#!/bin/bash
# DispatchLearn - Database Restore Script
# Target: RPO 15 minutes | RTO 4 hours

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/app/backups}"
DB_HOST="${DB_HOST:-mysql}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-dispatchlearn}"
DB_PASSWORD="${DB_PASSWORD:-dispatchlearn_secret}"
DB_NAME="${DB_NAME:-dispatchlearn}"

if [ -z "${1:-}" ]; then
  echo "Usage: restore.sh <backup_file.sql.gz>"
  echo ""
  echo "Available backups:"
  ls -lt "${BACKUP_DIR}"/full_*.sql.gz 2>/dev/null || echo "  No backups found"
  exit 1
fi

BACKUP_FILE="$1"

if [ ! -f "${BACKUP_FILE}" ]; then
  echo "[restore][error] Backup file not found: ${BACKUP_FILE}"
  exit 1
fi

echo "[restore][start] Starting restore from: ${BACKUP_FILE}"

# Verify checksum if available
CHECKSUM_FILE="${BACKUP_FILE}.sha256"
if [ -f "${CHECKSUM_FILE}" ]; then
  echo "[restore][verify] Verifying checksum..."
  if sha256sum -c "${CHECKSUM_FILE}" 2>/dev/null; then
    echo "[restore][verify] Checksum OK"
  else
    echo "[restore][error] Checksum verification FAILED"
    exit 1
  fi
else
  echo "[restore][warn] No checksum file found, skipping verification"
fi

# Restore
echo "[restore][db] Dropping and recreating database..."
mysql -h "${DB_HOST}" -P "${DB_PORT}" -u root -prootpassword \
  -e "DROP DATABASE IF EXISTS ${DB_NAME}; CREATE DATABASE ${DB_NAME};"

echo "[restore][db] Restoring from backup..."
gunzip -c "${BACKUP_FILE}" | mysql \
  -h "${DB_HOST}" \
  -P "${DB_PORT}" \
  -u "${DB_USER}" \
  -p"${DB_PASSWORD}" \
  "${DB_NAME}"

echo "[restore][done] Restore completed at $(date)"

# Validation
echo "[restore][validate] Running validation queries..."
TABLE_COUNT=$(mysql -h "${DB_HOST}" -P "${DB_PORT}" -u "${DB_USER}" -p"${DB_PASSWORD}" \
  -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${DB_NAME}'" 2>/dev/null)
echo "[restore][validate] Tables restored: ${TABLE_COUNT}"

AUDIT_COUNT=$(mysql -h "${DB_HOST}" -P "${DB_PORT}" -u "${DB_USER}" -p"${DB_PASSWORD}" \
  -N -e "SELECT COUNT(*) FROM ${DB_NAME}.audit_logs" 2>/dev/null || echo "0")
echo "[restore][validate] Audit log entries: ${AUDIT_COUNT}"

echo "[restore][complete] Restore validation complete"
