#!/bin/bash
#
# create_jwt_keys_and_secrets.sh generates encrypted JWT signer keys and uploads
# them as secrets in the GCP Secret Manager.

set -eu

PROJECT=${1:?please provide project}
KEYID=${2:?please provide keyid}

LOCATE_PRIVATE_KEY=jwk_sig_EdDSA_locate_${KEYID}
LOCATE_SECRET_NAME=locate-service-signer-key
MONITORING_PRIVATE_KEY=jwk_sig_EdDSA_monitoring_${KEYID}
MONITORING_SECRET_NAME=locate-monitoring-service-verify-key

# Get the project number for this project, which will be used to construct the
# default Cloud Build service account name for this project.
PROJECT_NUMBER=$(
  gcloud projects list --filter "project_id=${PROJECT}" \
      --format "value(project_number)"
)

which jwk-keygen &> /dev/null || \
    ( echo "Run: go get gopkg.in/square/go-jose.v2/jwk-keygen" && \
    exit 1 )

if [[ ! -f ${LOCATE_PRIVATE_KEY} ]] ; then
  # Create JWK key.
  echo "Creating private locate key: ${LOCATE_PRIVATE_KEY}"
  jwk-keygen --use=sig --alg=EdDSA --kid=locate_${KEYID}
fi

echo "Creating GCP secret for the private locate key."
gcloud secrets create "${LOCATE_SECRET_NAME}" \
      --data-file "${LOCATE_PRIVATE_KEY}" \
      --project "${PROJECT}"


# Allow the default Cloud Build service account to access the secret.
echo "Authorizing Cloud Build to access the locate secret."
gcloud secrets add-iam-policy-binding ${LOCATE_SECRET_NAME} \
      --member "serviceAccount:${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com" \
      --role "roles/secretmanager.secretAccessor" \
      --project "${PROJECT}"

if [[ ! -f ${MONITORING_PRIVATE_KEY} ]] ; then
  # Create JWK key.
  echo "Creating private monitoring key: ${MONITORING_PRIVATE_KEY}"
  jwk-keygen --use=sig --alg=EdDSA --kid=monitoring_${KEYID}
fi

echo "Creating GCP secret for the monitoring key."
gcloud secrets create "${MONITORING_SECRET_NAME}" \
      --data-file "${MONITORING_PRIVATE_KEY}" \
      --project "${PROJECT}"

# Allow the default Cloud Build service account to access the secret.
echo "Authorizing Cloud Build to access the monitoring secret."
gcloud secrets add-iam-policy-binding ${MONITORING_SECRET_NAME} \
      --member "serviceAccount:${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com" \
      --role "roles/secretmanager.secretAccessor" \
      --project "${PROJECT}"
