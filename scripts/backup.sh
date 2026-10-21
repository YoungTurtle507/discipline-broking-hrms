#!/usr/bin/env bash
# Scheduled backup: a compressed, timestamped Postgres dump plus a copy of
# backend/storage/ (face photos, employee/leave documents) — kept here under
# backups/ (already gitignored) with a rolling retention window.
#
# Usage: ./scripts/backup.sh   (intended to run on a schedule — see the cron
# example at the bottom of this file — but safe to run by hand any time)
#
# This alone only protects against "oops, bad data" (the same kind of
# pre-fix snapshot already sitting in backups/ from manual use). It does NOT
# protect against this machine's disk failing, since the backups live on
# that same disk — see BACKUP_REMOTE_DEST below to also copy them off this
# machine once you've picked a destination (another machine you control, or
# cloud object storage). Until that's set, this script still does the
# useful local-retention half on its own.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/scripts/dev-env.sh"

BACKUP_DIR="$REPO_ROOT/backups"
STORAGE_DIR="$REPO_ROOT/backend/storage"
STAMP="$(date +%Y%m%d-%H%M%S)"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"

mkdir -p "$BACKUP_DIR"

echo "Dumping database ($POSTGRES_DB)..."
DB_DUMP="$BACKUP_DIR/db-$STAMP.sql.gz"
PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  | gzip > "$DB_DUMP"
echo "  $DB_DUMP ($(du -h "$DB_DUMP" | cut -f1))"

STORAGE_ARCHIVE=""
if [ -d "$STORAGE_DIR" ] && [ -n "$(ls -A "$STORAGE_DIR" 2>/dev/null)" ]; then
  echo "Archiving backend/storage/ (face photos, documents)..."
  STORAGE_ARCHIVE="$BACKUP_DIR/storage-$STAMP.tar.gz"
  tar -czf "$STORAGE_ARCHIVE" -C "$REPO_ROOT/backend" storage
  echo "  $STORAGE_ARCHIVE ($(du -h "$STORAGE_ARCHIVE" | cut -f1))"
else
  echo "backend/storage/ is empty or missing — nothing to archive."
fi

# Optional off-machine copy — a no-op until this is actually set, since a
# backup sitting on the same disk as the live data isn't a real backup.
# Point it at anywhere rsync can reach: another machine you control
# ("user@host:/path/"), or a cloud-storage rclone/CLI target if you'd rather
# use that instead of rsync.
if [ -n "${BACKUP_REMOTE_DEST:-}" ]; then
  echo "Copying today's backups to $BACKUP_REMOTE_DEST..."
  if [ -n "$STORAGE_ARCHIVE" ]; then
    rsync -a "$DB_DUMP" "$STORAGE_ARCHIVE" "$BACKUP_REMOTE_DEST"
  else
    rsync -a "$DB_DUMP" "$BACKUP_REMOTE_DEST"
  fi
else
  echo "BACKUP_REMOTE_DEST is not set — backups are staying on this machine only."
  echo "Set it (e.g. in .env) once you've picked an off-site destination."
fi

echo "Removing backups older than $RETENTION_DAYS days..."
find "$BACKUP_DIR" -maxdepth 1 -type f \( -name 'db-*.sql.gz' -o -name 'storage-*.tar.gz' \) -mtime "+$RETENTION_DAYS" -print -delete

echo
echo "BACKUP-OK"

# --- Scheduling example (run: crontab -e) ---
# Daily at 2:17 AM (an off-the-hour minute, so it's not fighting every other
# server's cron jobs that fire at :00):
#   17 2 * * * /path/to/scripts/backup.sh >> /path/to/logs/backup.log 2>&1
