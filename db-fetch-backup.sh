#!/bin/bash

# Script to fetch backups from GCP
# Service account needs to be activated before running this script:
# gcloud auth activate-service-account --key-file /path/to/serviceaccountToken
#
# Created by: Youssef Bel Mekki and Sten Røkke, NAV IT

usage() {
  echo "------------------------------------------------------"
  echo "sh db-fetch-backup.sh [gcp-bucket] [backup-directory]"
  echo ""
  echo "gcp-backup: Source bucket in GCP to fetch files from"
  echo "backup-directory: Directory to store files from bucket"
  echo "------------------------------------------------------"
 }

#main

if [ "$#" -lt 2 ]; then
  usage
  exit 1
fi

SOURCE_BUCKET="$1"
BACKUP_DIR="$2"

if [[ "$(gsutil version)" =~ "gsutil version" && ! -f "/usr/local/bin/gsutil" ]]; then
  echo "Google Cloud SDK not installed"
  exit 1
fi

gsutil -m rsync -r ${SOURCE_BUCKET} ${BACKUP_DIR} || echo "File transfer failed" && exit 1

#TODO  Deletion of files in bucket could be done here if desired
