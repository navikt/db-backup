# db-backup

[![Github Actions](https://github.com/nais/db-backup/workflows/Build%20and%20deploy%20db-backup/badge.svg)](https://github.com/nais/db-backup/actions?query=workflow%3A%22Build+and+deploy+db-backup%22)
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/nais/named/master/LICENSE)

db-backup is docker container with a simple backup script for backing up all postgreSQL databases in the kubernetes cluster to a bucket.

### Requirements

  * Service account with project viewer role
  * JSON key created for the service account
  * K8s secret with JSON key mounted into the container

### Deployment

The github action builds and pushes the docker image and then updates the [navikt/nais-yaml](https://github.com/navikt/nais-yaml) repository.
