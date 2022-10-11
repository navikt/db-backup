#!/bin/bash

# Script to fetch bucket content from GCP
# Created by: Youssef Bel Mekki and Sten Røkke, NAV IT

usage() {
  echo "------------------------------------------------------------------"
  echo "sh bucket-fetch.sh [environment] [key file for service account]"
  echo ""
  echo "environment: dev or prod"
  echo "key file for sa: key file for service account in JSON format"
  echo "------------------------------------------------------------------"
 }

#main

if [ "$#" -lt 2 ]; then
  usage
  exit 1
fi

ENV="$1"
KEY_FILE="$2"
BACKUP_DIR="/pgbackup/gcp_backup/buckets/$ENV"
PATH=$PATH:$HOME/google-cloud-sdk/bin
CA_FILE="/etc/pki/tls/certs/ca-bundle.crt"
PROXY="http://webproxy-nais.nav.no:8088"
EXCLUDE_BUCKETS="gs://database-backup-dev/ gs://database-backup-prod/ gs://teamjobbsoker-datamesh-poc/ gs://petra-hr-chatbot-artifacts/ gs://petra-hr-chatbot-prod-40d5.appspot.com/"
DATE=$(date +%Y%m%d)
PASS=$(pwgen -s 22 1)

if [ ! -f "${KEY_FILE}" ]; then
  echo "No key file found"
  exit 1
fi

if [[ "$(gsutil version)" =~ "gsutil version" && ! -f "$HOME/google-cloud-sdk/bin/gsutil" ]]; then
  echo "Google Cloud SDK not installed"
  exit 1
fi

if [ "${ENV}" == "prod" ]; then
  DB_PROJECT_ID="team-database-prod-09b0"
else
  DB_PROJECT_ID="team-database-dev-988b"
fi

# Configuring proxy settings
gcloud config set core/custom_ca_certs_file ${CA_FILE}
export https_proxy=${PROXY}

# Activating service account
gcloud auth activate-service-account --key-file ${KEY_FILE}

# Get project list and assign to variable for iteration
gcloud projects list > project_list.txt

# Deleting header line
sed -i '1d' project_list.txt

while read PROJECT_ID PROJECT_NAME PROJECT_NUMBER; do
  gcloud config set project ${PROJECT_ID}
  BUCKETS=$(gsutil ls)
  if [ "${BUCKETS}" != "" ]; then
    for bucket in ${BUCKETS}; do
      if [[ "${EXCLUDE_BUCKETS}" != *"${bucket}"* ]]; then
        bucketName=$(echo $bucket | cut -d"/" -f 3)
        backupTarget=${BACKUP_DIR}/${PROJECT_NAME}/${DATE}/${bucketName}
        mkdir -p $backupTarget
        gsutil -m rsync -r $bucket $backupTarget
      fi
    done
  fi
done < project_list.txt

# Compressing files to save space
tar -czf ${BACKUP_DIR}/../bucket_${DATE}_${ENV}.tar.gz ${BACKUP_DIR}

# Encrypting file and storing secret to gcp
gpg --batch --yes --pinentry-mode=loopback --passphrase ${PASS} -c ${BACKUP_DIR}/../bucket_${DATE}_${ENV}.tar.gz
gcloud config set project ${DB_PROJECT_ID}
echo -n ${PASS}|base64 | gcloud secrets versions add b_enc_key --data-file=-
echo -n ${PASS}|base64 > ${BACKUP_DIR}/../buckets/.enc

# Removing unencrypted files
rm -r ${BACKUP_DIR}/../bucket_${DATE}_${ENV}.tar.gz
rm -r ${BACKUP_DIR}