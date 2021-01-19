#!/bin/bash

if [ "$BUCKET_NAME" == "" ]; then
  echo "Bucket name not set"
  exit 1
fi

get_project_id() {
  namespace="$1"
  local project_id
  project_id=$(kubectl get namespace "$namespace" -o jsonpath='{.metadata.annotations.cnrm\.cloud\.google\.com/project-id}')
  echo "$project_id"
}

activate_service_account_in() {
  namespace="$1"
  gcloud auth activate-service-account --key-file /credentials/saKey
  gcloud config set project "$(get_project_id "$namespace")"
}

backup_in() {
  instance="$1"
  namespace="$2"

  if [[ "$instance" == "" || "$namespace" == "" ]]; then
    echo "Namespace or instance not set"
    exit 1
  fi

  echo "$(date +%H%M%S): Backing up instance $instance"
  service_account_email=$(kubectl get sqlinstance "$instance" -n "$namespace" --no-headers -o custom-columns=":status.serviceAccountEmailAddress")
  dump_file_name="$(date +%Y%m%d)_${instance}"
  gsutil iam ch serviceAccount:"${service_account_email}":objectCreator gs://"$BUCKET_NAME"
  gsutil -q stat gs://"$BUCKET_NAME"/"$namespace"/"$dump_file_name"

  if [ "$?" != 0 ]; then
    # TODO: Consider using --offload for less disruption
    gcloud sql export sql "$instance" gs://"$BUCKET_NAME"/"$namespace"/"$dump_file_name" --database="$db"
  fi
  # TODO: could save failing command to list and run list if not empty again. to handle updates interrupts recursively?.
}

# main
all_db_namespaces=$(kubectl get sqldatabases -A --no-headers -o custom-columns=":metadata.namespace" | uniq)

for namespace in ${all_db_namespaces}; do
  echo "Getting instances in namespace $namespace"
  dbs=$(kubectl get sqldatabases -n "$namespace" --no-headers -o custom-columns=":metadata.name")
  activate_service_account_in "$namespace"

  for db in $dbs; do
    instance=$(kubectl get sqldatabase "$db" -n "$namespace" --no-headers -o custom-columns=":spec.instanceRef.name")
    verifyInstance=$(kubectl get sqlinstance -n "$namespace" "$instance" >/dev/null 2>&1)

    if [ "$?" != 0 ]; then
      echo "spec.instanceRef.name in database $db does not exist. Skipping instance $instance..."
      # TODO Should handle db changed manually
    else
      backup_in "$instance" "$namespace"
    fi
  done
done
