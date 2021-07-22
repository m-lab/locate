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

GAE_SERVICE_ACCOUNT="serviceAccount:${PROJECT}@appspot.gserviceaccount.com"

which jwk-keygen &> /dev/null || \
    ( echo "Run: go get gopkg.in/square/go-jose.v2/jwk-keygen" && \
    exit 1 )


# Checks if a secret exists or not.
function secretExists() {
  local name=$1
  existing_secret=$(
    gcloud secrets list --filter "name:$1" \
          --format "value(name)" \
          --project mlab-sandbox
  )
  if [[ -n $existing_secret ]]; then
    echo "Secret '${name}' already exists."
    return 0
  else
    return 1
  fi
}

function addSecret() {
  local name=$1
  local file=$2
  gcloud secrets create "${name}" \
        --data-file "${file}" \
        --project "${PROJECT}"
}

function addNewVersion() {
  local name=$1
  local file=$2
  local disable=$3
  gcloud secrets versions add "${name}" \
        --data-file "${file}" \
        --project "${PROJECT}"

  if [[ $disable != "disable" ]]; then
    return
  fi

  version=$(
    gcloud secrets versions list "${name}" --limit 1 \
          --format "value(name)" \
          --project mlab-sandbox
  )

  gcloud secrets versions disable "${version}" \
        --secret "${name}" \
        --project "${PROJECT}"
}

# Checks if an IAM policy binding already exists for the specified secret name.
function iamPolicyBindingExists() {
  local name=$1
  if gcloud secrets get-iam-policy "${name}" \
          --project "${PROJECT}" \
          | grep --quiet "${GAE_SERVICE_ACCOUNT}"; then
    echo "IAM policy binding already exists for secret: ${name}"
    return 0
  else
    return 1
  fi
}

# Adds an IAM policy binding for the specified secret name, allowing the default
# AppEngine service account to access the secret.
function addIAMPolicyBinding() {
  local name=$1
  gcloud secrets add-iam-policy-binding ${name} \
        --member "${GAE_SERVICE_ACCOUNT}" \
        --role "roles/secretmanager.secretAccessor" \
        --project "${PROJECT}"
}

# Create JWT signer key.
if [[ ! -f ${LOCATE_PRIVATE_KEY} ]] ; then
  echo "Creating private locate signer key: ${LOCATE_PRIVATE_KEY}"
  jwk-keygen --use=sig --alg=EdDSA --kid=locate_${KEYID}
fi

# Create secret for the JWT signer key.
if ! secretExists "${LOCATE_SECRET_NAME}"; then
  addSecret "${LOCATE_SECRET_NAME}" "${LOCATE_PRIVATE_KEY}"
else
  cat <<EOF

Would you like to create a new version of this secret?

WARNING: the new secret version will be disabled. Consult the README file of
this repo for further instructions on rotating keys. DO NOT ENABLE the new
secret version until you have completed all the steps in the documentation. The
public key will be found in this directory after this script has completed.

Are you sure you want to continue? [y/N]:
EOF
  read addversion
  if [[ "${addversion}" == "y" ]]; then
    addNewVersion "${LOCATE_SECRET_NAME}" "${LOCATE_PRIVATE_KEY}" "disable"
  fi
fi

if ! iamPolicyBindingExists "${LOCATE_SECRET_NAME}"; then
  addIAMPolicyBinding "${LOCATE_SECRET_NAME}"
fi

# Create JWT monitoring keys.
if [[ ! -f ${MONITORING_PRIVATE_KEY} ]] ; then
  echo "Creating monitoring keys: ${MONITORING_PRIVATE_KEY}"
  jwk-keygen --use=sig --alg=EdDSA --kid=monitoring_${KEYID}
fi

# Create secret for the JWT monitoring key.
if ! secretExists "${MONITORING_SECRET_NAME}"; then
  addSecret "${MONITORING_SECRET_NAME}" "${MONITORING_PRIVATE_KEY}"
else
  cat <<EOF

Would you like to create a new version of this secret?

WARNING: the new secret version will be created for the monitoring verify key,
but will do nothing if the corresponding private key is not updated. See the
README of this repo for documentation on rotating monitoring keys.

Are you sure you want to continue? [y/N]:
EOF
  read addversion
  if [[ "${addversion}" == "y" ]]; then
    addNewVersion "${MONITORING_SECRET_NAME}" "${MONITORING_PRIVATE_KEY}" "enable"
  fi
fi

if ! iamPolicyBindingExists "${MONITORING_SECRET_NAME}"; then
  addIAMPolicyBinding "${MONITORING_SECRET_NAME}"
fi
