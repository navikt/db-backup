FROM google/cloud-sdk:532.0.0-stable
ENV KUBE_VERSION=v1.31.0
RUN apt update && apt install bash
ADD https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/bin/linux/amd64/kubectl /usr/local/bin/kubectl
RUN chmod +x /usr/local/bin/kubectl
RUN echo '[GoogleCompute]\nservice_account = default' > /etc/boto.cfg
COPY db-backup.sh /usr/bin/
ENTRYPOINT [ "/usr/bin/db-backup.sh"]
