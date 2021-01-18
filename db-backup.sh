#!/bin/bash

if [ $# -lt 1 ]; then
  echo "Must specify two arguments: bucket and cluster"
  exit 1
fi

BUCKET=$1

all_db_namespaces=$(kubectl get sqldatabases -A --no-headers -o custom-columns=":metadata.namespace" | uniq)

for namespace in ${all_db_namespaces}; do
  echo "Getting instances in namespace $namespace"
  dbs=$(kubectl get sqldatabases -n $namespace --no-headers -o custom-columns=":metadata.name")
  project_id=$(kubectl get namespace $namespace -o jsonpath='{.metadata.annotations.cnrm\.cloud\.google\.com/project-id}')
  gcloud auth activate-service-account --key-file /credentials/saKey
  gcloud config set project "$project_id"
  echo "project_id: ${project_id}"
  for db in $dbs; do
    instance=$(kubectl get sqldatabase $db -n $namespace --no-headers -o custom-columns=":spec.instanceRef.name")
    verifyInstance=$(kubectl get sqlinstance -n $namespace $instance > /dev/null 2>&1)
    if [ $? != 0 ]; then
      echo "spec.instanceRef.name in database $db does not exist. Skipping instance $instance..."
    else
      echo "Backing up instance: $instance"
      service_account_email=$(kubectl get sqlinstance $instance -n $namespace --no-headers -o custom-columns=":status.serviceAccountEmailAddress")
      dump_file_name="$(date +%Y%m%d%H%M%S)_${instance}_${project_id}"
      gsutil iam ch serviceAccount:"${service_account_email}":objectCreator gs://"$BUCKET"
      # TODO: Consider using --offload for less disruption
      gcloud sql export sql "${instance}" gs://"$BUCKET"/"$namespace"/"$dump_file_name" --database="$db"
    fi
  done
done
