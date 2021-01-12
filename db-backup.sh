#!/bin/bash -e

if [ $# -lt 2 ]; then
  echo "Missing parameters bucket and cluster"
  exit 1
fi

if [[ ! "$2" =~ ^("dev-gcp"|"prod-gcp")$ ]]; then
  echo "Invalid cluster name: 'dev-gcp' | 'prod-gcp'"
  exit 1
fi

BUCKET=$1
CLUSTER=$2

# git clone https://github.com/navikt/kubeconfigs.git
echo "Setting cluster"
kubectl config set-context "$CLUSTER"
echo "Getting all namespaces"
all_db_namespaces=$(kubectl get sqldatabases -A --no-headers -o custom-columns=":metadata.namespace" | uniq)

#for namespace in ${all_db_namespaces}; do
for namespace in aura; do
  echo "Getting instances in namespace $namespace"
  dbs=$(kubectl get sqldatabases -n $namespace --no-headers -o custom-columns=":metadata.name")
  project_id=$(kubectl get namespace $namespace -o jsonpath='{.metadata.annotations.cnrm\.cloud\.google\.com/project-id}')
  gcloud config set project "$project_id"
  echo "project_id: ${project_id}"
  for db in $dbs; do
    instance=$(kubectl get sqldatabase $db -n $namespace --no-headers -o custom-columns=":spec.instanceRef.name")
    echo "Instance name: $instance"
    service_account_email=$(kubectl get sqlinstance $instance -n $namespace --no-headers -o custom-columns=":status.serviceAccountEmailAddress")
    echo "serviceAccountEmail: ${service_account_email}"
    dump_file_name="$(date +%Y%m%d)_${instance}_${project_id}"
    echo $dump_file_name
    gsutil iam ch serviceAccount:"${service_account_email}":objectCreator gs://"$BUCKET"
    gcloud sql export sql "${instance}" gs://"$BUCKET"/"$dump_file_name" --database="$db"
  done
done
