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
#
# AUTHENTICATION: the endpoint is protected by an API key, which the job sends
# in an X-API-Key header. This script deliberately does not manage that header.
# The key is set once per project by hand, and no header flags are passed here so
# that a deploy cannot overwrite or erase it. Consequently a newly created job
# has no key and will receive 401 until someone adds one:
#
#   gcloud --project <project> scheduler jobs update http prometheus \
#       --location <location> --update-headers X-API-Key=<key>

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

if gcloud --project "${PROJECT}" scheduler jobs describe prometheus \
    --location "${LOCATION}" > /dev/null 2>&1; then
  verb="update"
  # The job authenticates with a header, so it should carry no OIDC or OAuth
  # token. --clear-auth-token removes one left by an earlier configuration and is
  # harmless when none is set. Header flags are intentionally omitted so that the
  # hand-managed X-API-Key survives.
  extra_args=(--clear-auth-token)
else
  verb="create"
  extra_args=()
fi

gcloud --project "${PROJECT}" scheduler jobs "${verb}" http prometheus \
  --location "${LOCATION}" \
  --schedule "* * * * *" \
  --time-zone "Etc/GMT" \
  --uri "${URI}" \
  --http-method GET \
  --attempt-deadline 180s \
  --description "Refreshes locate Prometheus health signals (managed by CI)" \
  "${extra_args[@]}"
