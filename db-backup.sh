#!/bin/bash

if [ "$BUCKET_NAME" == "" ]; then
  echo "bucket name not set"
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

  gcloud config set project "$teamProjectId"
}

backupInstance() {
  db="$1"
  instance="$2"
  namespace="$3"

  if [[ "$instance" == "" || "$namespace" == "" ]]; then
    echo "namespace or instance not set"
    exit 1
  fi

  echo "$(date +%H%M%S): backing up instance $instance"
  service_account_email=$(kubectl get sqlinstance "$instance" -n "$namespace" --no-headers -o custom-columns=":status.serviceAccountEmailAddress")
  dump_file_name="$(date +%Y%m%d)_${instance}"
  
  echo "setting permissions for $service_account_email"
  gcloud storage buckets add-iam-policy-binding  gs://"${BUCKET_NAME}" --member=serviceAccount:"${service_account_email}" --role=roles/storage.objectCreator

  echo "checking if $bucketTarget exists before backing up"
  bucketTarget=gs://"$BUCKET_NAME"/"$namespace"/"$dump_file_name".gz
  if ! gcloud storage ls "$bucketTarget" 2>/dev/null; then
      gcloud sql export sql "$instance" "$bucketTarget" --database="$db" --offload --async
  fi

  echo "removing permissions for $service_account_email"
  gcloud storage buckets remove-iam-policy-binding  gs://"${BUCKET_NAME}" --member=serviceAccount:"${service_account_email}" --role=roles/storage.objectCreator
}

watchOperationForInstance() {
  instance="$1"

  if [[ "$instance" == "" ]]; then
    echo "namespace or instance not set"
    exit 1
  fi

  PENDING_OPERATIONS=$(gcloud sql operations list --instance="$instance" --filter='status!=DONE' --format='value(name)')
  if [ "$PENDING_OPERATIONS" != "" ]; then
    echo "waiting for operations to finish"
    gcloud sql operations wait "${PENDING_OPERATIONS}" --timeout=72000
  fi
}

verifyInstance() {
namespace="$1"
instance="$2"
  if [[ "$instance" == "" || "$namespace" == "" ]]; then
    echo "namespace or instance not set"
    exit 1
  fi

  kubectl get sqlinstance -n "$namespace" "$instance" >/dev/null 2>&1

  return $?
}


# main
all_db_namespaces=$(kubectl get sqldatabases -A --no-headers -o custom-columns=":metadata.namespace" | uniq)

for namespace in ${all_db_namespaces}; do
  echo "getting instances in namespace $namespace"
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

    if verifyInstance "$namespace" "$instance"; then
      backupInstance "$db" "$instance" "$namespace"
    else
      echo "instance $instance referenced in database $db does not exist. Skipping instance."
    fi
  done

done

for namespace in ${all_db_namespaces}; do
  dbs=$(kubectl get sqldatabases -n "$namespace" --no-headers -o custom-columns=":metadata.name")
  activateSA "$namespace"

  for db in $dbs; do
    instance=$(kubectl get sqldatabase "$db" -n "$namespace" --no-headers -o custom-columns=":spec.instanceRef.name")

    if verifyInstance "$namespace" "$instance"; then
      watchOperationForInstance "$instance"
    else
      echo "instance $instance referenced in database $db does not exist. Skipping instance."
    fi
  done

done
