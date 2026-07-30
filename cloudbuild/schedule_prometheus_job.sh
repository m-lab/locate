#!/bin/bash
#
# schedule_prometheus_job.sh reconciles the "prometheus" Cloud Scheduler job.
#
# The job calls /v2/platform/prometheus once per minute, which makes locate
# re-query the platform Prometheus for script_success and gmx_machine_maintenance
# and cache the result in Memorystore. Server selection uses that cache to
# exclude machines failing end-to-end probes or flagged for maintenance.
#
# The job is updated in place when it already exists and created when it does
# not, so each deploy converges any manual drift without a window in which the
# job is absent.
#
# The target URL is derived from the project so that it always matches the host
# in openapi.yaml and endpoints_api_service.name in app.yaml. A hand-created
# version of this job used "locate.<project>.appspot.com", which cannot match
# App Engine's single-label wildcard certificate, so Cloud Scheduler failed TLS
# validation and the request never reached the service.

set -euxo pipefail

PROJECT=${1:?Please provide a project}

# Cloud Scheduler jobs live in the region of the project's App Engine app. This
# cannot be read from "gcloud app describe", which reports "us-central" for
# mlab-ns while Cloud Scheduler requires "us-central1".
case "${PROJECT}" in
  mlab-sandbox)
    LOCATION="us-east1"
    ;;
  mlab-staging|mlab-ns)
    LOCATION="us-central1"
    ;;
  *)
    echo "No Cloud Scheduler location known for ${PROJECT}; skipping." >&2
    exit 0
    ;;
esac

# Must match "host" in openapi.yaml and endpoints_api_service.name in app.yaml.
URI="https://locate-dot-${PROJECT}.appspot.com/v2/platform/prometheus"

# The App Engine default service account, which is also the issuer configured
# for scheduler_oidc in openapi.yaml.
SERVICE_ACCOUNT="${PROJECT}@appspot.gserviceaccount.com"

# Must match x-google-audiences for scheduler_oidc in openapi.yaml.
AUDIENCE="locate-prometheus"

# "update" merges headers rather than replacing them, so an existing job needs
# its headers cleared explicitly to drop the X-API-Key left by the hand-created
# version. Cloud Scheduler sets its own User-Agent, so no custom headers are
# needed. "create" has no --clear-headers flag and starts with none.
if gcloud --project "${PROJECT}" scheduler jobs describe prometheus \
    --location "${LOCATION}" > /dev/null 2>&1; then
  verb="update"
  header_args=(--clear-headers)
else
  verb="create"
  header_args=()
fi

gcloud --project "${PROJECT}" scheduler jobs "${verb}" http prometheus \
  --location "${LOCATION}" \
  --schedule "* * * * *" \
  --time-zone "Etc/GMT" \
  --uri "${URI}" \
  --http-method GET \
  --attempt-deadline 180s \
  --oidc-service-account-email "${SERVICE_ACCOUNT}" \
  --oidc-token-audience "${AUDIENCE}" \
  --description "Refreshes locate Prometheus health signals (managed by CI)" \
  "${header_args[@]}"
