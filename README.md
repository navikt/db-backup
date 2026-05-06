# db-backup

[![Build and deploy db-backup](https://github.com/navikt/db-backup/actions/workflows/main.yml/badge.svg)](https://github.com/navikt/db-backup/actions/workflows/main.yml)
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/nais/named/master/LICENSE)

db-backup is a Go application that backs up all Cloud SQL PostgreSQL databases in the Kubernetes cluster to a GCS bucket. It runs as a Naisjob (CronJob) and uses the Cloud SQL Admin API to export databases.

## How it works

1. Lists all `SQLDatabase` custom resources (CNRM) across the cluster
2. For each database, resolves the GCP project from the namespace annotation (`cnrm.cloud.google.com/project-id`)
3. Grants `roles/storage.objectCreator` to the Cloud SQL service account on the target bucket
4. Checks if today's backup already exists (idempotent)
5. Starts an async SQL export to `gs://<bucket>/<namespace>/<YYYYMMDD>_<instance>.gz`
6. Waits for all export operations to complete

## Requirements

- Workload identity (no JSON keys) — the Naisjob service account must have:
  - `roles/cloudsql.admin` on team GCP projects (to trigger exports)
  - `roles/storage.admin` on the backup bucket (to manage IAM and check objects)
- ClusterRole with `list`/`get` on `namespaces`, `sqldatabases`, and `sqlinstances` (see `nais/clusterrole.yaml`)

## Development

Requires [mise](https://mise.jdx.dev/) — run `mise install` to set up Go.

```bash
# Build
mise run build

# Run tests
mise run test

# Lint (requires golangci-lint v2.x)
mise run lint

# Run all checks (lint + test + build)
mise run check

# Build Docker image
mise run docker-build

# Clean build artifacts
mise run clean
```

## Deployment

The GitHub Actions workflow:
1. Runs tests and golangci-lint
2. Builds and signs the container image (pushed to GAR)
3. Deploys to `prod-gcp` as a Naisjob

## Verifying db-backup image and its contents

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

## Project structure

```
cmd/db-backup/main.go       # Entrypoint
internal/
  backup/backup.go          # Orchestration logic
  gcp/client.go             # GCP client initialization
  gcp/sqladmin.go           # Cloud SQL Admin API (export, operations)
  gcp/storage.go            # GCS object existence check
  gcp/iam.go                # Bucket IAM policy management
  k8s/k8s.go                # Kubernetes dynamic client for CNRM CRDs
nais/
  naisjob.yaml              # Naisjob manifest
  clusterrole.yaml          # RBAC for the service account
  alert.yaml                # PrometheusRule alerts
```
