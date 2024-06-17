#!/bin/bash

if [ "$BUCKET_NAME" == "" ]; then
  echo "$(date +%H%M%S): bucket name not set"
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
    echo "$(date +%H%M%S): namespace or instance not set"
    exit 1
  fi

  service_account_email=$(kubectl get sqlinstance "$instance" -n "$namespace" --no-headers -o custom-columns=":status.serviceAccountEmailAddress")
  dump_file_name="$(date +%Y%m%d)_${instance}"
  
  echo "$(date +%H%M%S): setting permissions for $service_account_email"
  gcloud storage buckets add-iam-policy-binding gs://"${BUCKET_NAME}" --member=serviceAccount:"${service_account_email}" --role=roles/storage.objectCreator 1>/dev/null

  bucketTarget=gs://"$BUCKET_NAME"/"$namespace"/"$dump_file_name".gz
  if ! gcloud storage ls "$bucketTarget" 1>&2>/dev/null; then
    echo "$(date +%H%M%S): backing up instance $instance"
    gcloud sql export sql "$instance" "$bucketTarget" --database="$db" --offload --async
  else
    echo "$(date +%H%M%S): file already exists, skipping backup"
  fi
}

watchOperationForInstance() {
  instance="$1"

  if [[ "$instance" == "" ]]; then
    echo "$(date +%H%M%S): namespace or instance not set"
    exit 1
  fi

  PENDING_OPERATIONS=$(gcloud sql operations list --instance="$instance" --filter='status!=DONE' --format='value(name)')
  if [ "$PENDING_OPERATIONS" != "" ]; then
    echo "$(date +%H%M%S): waiting for operations to finish"
    gcloud sql operations wait "${PENDING_OPERATIONS}" --timeout=72000
  fi
}

verifyInstance() {
namespace="$1"
instance="$2"
  if [[ "$instance" == "" || "$namespace" == "" ]]; then
    echo "$(date +%H%M%S): namespace or instance not set"
    exit 1
  fi

  kubectl get sqlinstance -n "$namespace" "$instance" >/dev/null 2>&1

  return $?
}


# main
all_db_namespaces=$(kubectl get sqldatabases -A --no-headers -o custom-columns=":metadata.namespace" | uniq)

for namespace in ${all_db_namespaces}; do
  echo "$(date +%H%M%S): getting instances in namespace $namespace"
  k8sSqlDatabases=$(kubectl get sqldatabases -n "$namespace" --no-headers -o custom-columns=":metadata.name")
  k8sSqlInstances=$(kubectl get sqlinstances -n "$namespace" --no-headers -o custom-columns=":metadata.name")

  if [ "$(echo "$k8sSqlDatabases"|wc -l)" != "$(echo "$k8sSqlInstances"|wc -l)" ]; then
    echo "$(date +%H%M%S): mismatch in number of databases and instances in $namespace:"
    printf "Databases:\n%s" "$k8sSqlDatabases"
    printf "Instances:\n%s" "$k8sSqlInstances"
  fi

  activateSA "$namespace"

  for k8sSqlDatabase in $k8sSqlDatabases; do
    instanceRef=$(kubectl get sqldatabase "$k8sSqlDatabase" -n "$namespace" --no-headers -o custom-columns=":spec.instanceRef.name")
    pgDbName=$(kubectl get sqldatabase "$k8sSqlDatabase" -n "$namespace" --no-headers -o custom-columns=":spec.resourceID")

    if verifyInstance "$namespace" "$instanceRef"; then
      backupInstance "$pgDbName" "$instanceRef" "$namespace"
    else
      echo "$(date +%H%M%S): instance $instanceRef referenced in database $k8sSqlDatabase does not exist. Skipping instance."
    fi
  done

done

for namespace in ${all_db_namespaces}; do
  k8sSqlDatabases=$(kubectl get sqldatabases -n "$namespace" --no-headers -o custom-columns=":metadata.name")
  activateSA "$namespace"

  for k8sSqlDatabase in $k8sSqlDatabases; do
    instanceRef=$(kubectl get sqldatabase "$k8sSqlDatabase" -n "$namespace" --no-headers -o custom-columns=":spec.instanceRef.name")

    if verifyInstance "$namespace" "$instanceRef"; then
      watchOperationForInstance "$instanceRef"
    else
      echo "$(date +%H%M%S): instance $instanceRef referenced in database $k8sSqlDatabase does not exist. Skipping instance."
    fi
  done

done
