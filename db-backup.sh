#!/bin/bash

if [ $# -lt 2 ]; then
  echo "Must specify two arguments: bucket and cluster"
  exit 1
fi

if [[ ! "$2" =~ ^("dev-gcp"|"prod-gcp")$ ]]; then
  echo "Invalid cluster name: '$2'. Valid: 'dev-gcp' | 'prod-gcp'"
  exit 1
fi

BUCKET=$1
CLUSTER=$2

echo "Setting cluster"
kubectl config set-context "$CLUSTER"
echo "Getting all namespaces"
all_instance_namespaces=$(kubectl get sqlinstance -A --no-headers -o custom-columns=":metadata.namespace" | uniq)

#for namespace in ${all_instance_namespaces}; do
for namespace in aura; do
  echo "Getting instances in namespace $namespace"
  intances=$(kubectl get sqlinstance -n $namespace --no-headers -o custom-columns=":metadata.name")
  project_id=$(kubectl get namespace $namespace -o jsonpath='{.metadata.annotations.cnrm\.cloud\.google\.com/project-id}')
  gcloud auth activate-service-account --key-file /credentials/saKey
  gcloud config set project "$project_id"
  echo "project_id: ${project_id}"
  for instance in $instances; do
    echo "Instance name: $instance"
    service_account_email=$(kubectl get sqlinstance $instance -n $namespace --no-headers -o custom-columns=":status.serviceAccountEmailAddress")
    echo "serviceAccountEmail: ${service_account_email}"
    dump_file_name="$(date +%Y%m%d%H%M%S)_${instance}_${project_id}"
    echo $dump_file_name
    gsutil iam ch serviceAccount:"${service_account_email}":objectAdmin gs://"$BUCKET"
    gcloud sql export sql "${instance}" gs://"$BUCKET"/"$dump_file_name" --database="$db"
  done
done

