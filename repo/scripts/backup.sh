#!/bin/bash
# DispatchLearn - Database Backup Script
# Supports nightly full backup with binlog retention (15 min)
# RPO: 15 minutes | RTO: 4 hours

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/app/backups}"
DB_HOST="${DB_HOST:-mysql}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-dispatchlearn}"
DB_PASSWORD="${DB_PASSWORD:-dispatchlearn_secret}"
DB_NAME="${DB_NAME:-dispatchlearn}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo "[backup][start] Starting backup at $(date)"

mkdir -p "${BACKUP_DIR}"

# Full database dump
DUMP_FILE="${BACKUP_DIR}/full_${DB_NAME}_${TIMESTAMP}.sql"
mysqldump \
  -h "${DB_HOST}" \
  -P "${DB_PORT}" \
  -u "${DB_USER}" \
  -p"${DB_PASSWORD}" \
  --single-transaction \
  --routines \
  --triggers \
  --events \
  "${DB_NAME}" > "${DUMP_FILE}"

# Compress
gzip "${DUMP_FILE}"
DUMP_FILE="${DUMP_FILE}.gz"

# Generate checksum
sha256sum "${DUMP_FILE}" > "${DUMP_FILE}.sha256"

echo "[backup][done] Backup created: ${DUMP_FILE}"
echo "[backup][checksum] $(cat ${DUMP_FILE}.sha256)"

# Cleanup old backups
echo "[backup][retention] Removing backups older than ${RETENTION_DAYS} days"
find "${BACKUP_DIR}" -name "full_*.sql.gz" -mtime +${RETENTION_DAYS} -delete
find "${BACKUP_DIR}" -name "full_*.sql.gz.sha256" -mtime +${RETENTION_DAYS} -delete

echo "[backup][complete] Backup completed at $(date)"
