#!/bin/bash

if [ "$BUCKET_NAME" == "" ]; then
  echo "Bucket name not set"
  exit 1
fi

getProjectId() {
  namespace="$1"
  local project_id
  project_id=$(kubectl get namespace "$namespace" -o jsonpath='{.metadata.annotations.cnrm\.cloud\.google\.com/project-id}')
  echo "$project_id"
}

activateSA() {
  teamProjectId=$(getProjectId "$1")

  gcloud projects add-iam-policy-binding --role=roles/cloudsql.editor --member=serviceAccount:nais-dev-25b2.svc.id.goog[nais/db-backup-new] $teamProjectId
  gcloud config set project "$teamProjectId"
}

backupInstance() {
  db="$1"
  instance="$2"
  namespace="$3"

  if [[ "$instance" == "" || "$namespace" == "" ]]; then
    echo "Namespace or instance not set"
    exit 1
  fi

  echo "$(date +%H%M%S): Backing up instance $instance"
  service_account_email=$(kubectl get sqlinstance "$instance" -n "$namespace" --no-headers -o custom-columns=":status.serviceAccountEmailAddress")
  dump_file_name="$(date +%Y%m%d)_${instance}"
  gsutil iam ch serviceAccount:"${service_account_email}":objectCreator gs://"$BUCKET_NAME"
  bucketTarget=gs://"$BUCKET_NAME"/"$namespace"/"$dump_file_name".gz
  gsutil -q stat "$bucketTarget"

  if [ "$?" != 0 ]; then
      gcloud sql export sql "$instance" "$bucketTarget" --database="$db" --offload --async
  fi
}

watchOperationForInstance() {
  instance="$1"

  if [[ "$instance" == "" ]]; then
    echo "Namespace or instance not set"
    exit 1
  fi

  PENDING_OPERATIONS=$(gcloud sql operations list --instance="$instance" --filter='status!=DONE' --format='value(name)')
  if [ "$PENDING_OPERATIONS" != "" ]; then 
    gcloud sql operations wait "${PENDING_OPERATIONS}" --timeout=72000
  fi
}


# main
all_db_namespaces=$(kubectl get sqldatabases -A --no-headers -o custom-columns=":metadata.namespace" | uniq)

for namespace in ${all_db_namespaces}; do
  echo "Getting instances in namespace $namespace"
  dbs=$(kubectl get sqldatabases -n "$namespace" --no-headers -o custom-columns=":metadata.name")
  instances=$(kubectl get sqlinstances -n "$namespace" --no-headers -o custom-columns=":metadata.name")

  if [ "$(echo $dbs|wc -l)" != "$(echo $instances|wc -l)" ]; then
    echo "mismatch in number of databases and instances in $namespace:"
    echo "Databases:\n$dbs"
    echo "Instances:\n$instances"
  fi

  activateSA "$namespace"

  for db in $dbs; do
    instance=$(kubectl get sqldatabase "$db" -n "$namespace" --no-headers -o custom-columns=":spec.instanceRef.name")

    if [ "$(kubectl get sqlinstance -n "$namespace" "$instance" >/dev/null 2>&1)" != 0 ]; then
      echo "spec.instanceRef.name in database $db does not exist. Skipping instance $instance..."
    else
      backupInstance "$db" "$instance" "$namespace"
    fi
  done

done

for namespace in ${all_db_namespaces}; do
  echo "Getting instances in namespace $namespace"
  dbs=$(kubectl get sqldatabases -n "$namespace" --no-headers -o custom-columns=":metadata.name")
  activateSA "$namespace"

  for db in $dbs; do
    instance=$(kubectl get sqldatabase "$db" -n "$namespace" --no-headers -o custom-columns=":spec.instanceRef.name")

    if [ "$(kubectl get sqlinstance -n "$namespace" "$instance" >/dev/null 2>&1)" != 0 ]; then
      echo "spec.instanceRef.name in database $db does not exist. Skipping instance $instance..."
    else
      watchOperationForInstance "$instance"
    fi
  done

done
