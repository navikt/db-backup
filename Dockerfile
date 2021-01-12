FROM google/cloud-sdk:322.0.0-alpine
ENV KUBE_VERSION v1.17.3
RUN apk add --no-cache --update bash
ADD https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/bin/linux/amd64/kubectl /usr/local/bin/kubectl
RUN chmod +x /usr/local/bin/kubectl
COPY db-backup.sh /usr/bin/
ENTRYPOINT "/usr/bin/db-backup.sh database-backup-dev dev-gcp"