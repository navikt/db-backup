FROM google/cloud-sdk:541.0.0-stable
ENV KUBE_VERSION=v1.33.4
ENV TZ=Europe/Oslo
RUN apt update -y && apt install bash wget -y
RUN wget -O /usr/local/bin/kubectl https://dl.k8s.io/release/${KUBE_VERSION}/bin/linux/amd64/kubectl \
    && chmod +x /usr/local/bin/kubectl
#ADD https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/bin/linux/amd64/kubectl /usr/local/bin/kubectl
RUN chmod +x /usr/local/bin/kubectl
RUN echo '[GoogleCompute]\nservice_account = default' > /etc/boto.cfg
COPY db-backup.sh /usr/bin/
ENTRYPOINT [ "/usr/bin/db-backup.sh"]
