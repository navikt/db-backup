# db-backup

[![Github Actions](https://github.com/nais/db-backup/workflows/Build%20and%20deploy%20db-backup/badge.svg)](https://github.com/nais/db-backup/actions?query=workflow%3A%22Build+and+deploy+db-backup%22)
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/nais/named/master/LICENSE)

db-backup is docker container with a simple backup script for backing up all postgreSQL databases in the kubernetes cluster to a bucket.

### Requirements

  * Service account with project viewer role
  * JSON key created for the service account
  * k8s secret with JSON key mounted into the container

### Deployment

The github action builds and pushes the docker image and then updates the [navikt/nais-yaml](https://github.com/navikt/nais-yaml) repository.

### Verifying db-backup image and its contents

The image is signed "keylessly" using [Sigstore cosign](https://github.com/sigstore/cosign).
To verify its authenticity run
```
cosign verify \
--certificate-identity "https://github.com/nais/db-backup/.github/workflows/main.yml@refs/heads/main" \
--certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
ghcr.io/nais/db-backup/db-backup@sha256:<shasum>
```

The images are also attested with SBOMs in the [CycloneDX](https://cyclonedx.org/) format.
You can verify these by running
```
cosign verify-attestation --type cyclonedx \
--certificate-identity "https://github.com/nais/db-backup/.github/workflows/main.yml@refs/heads/main" \
--certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
ghcr.io/nais/db-backup/db-backup@sha256:<shasum>
```
