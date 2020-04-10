#!/bin/bash
#
# create_encrypted_signer_key.sh generates encrypted JWT signer keys and
# creates the necessary KMS keyring and key.

set -eu

PROJECT=${1:?please provide project}
KEYID=${2:?please provide keyid}

GCPARGS="--project ${PROJECT}"

KEYRING=locate-signer
KEY=jwk

# Create keyring if it's not already present.
keyring=$(
  gcloud ${GCPARGS} kms keyrings list \
    --location global \
    --format='value(name)' \
    --filter "name~.*/${KEYRING}$" )
if [[ -z ${keyring} ]] ; then
  echo "Creating keyring: ${KEYRING}"
  gcloud ${GCPARGS} kms keyrings create ${KEYRING} \
    --location=global
fi

# Create key within keyring if it's not already present.
key=$(
  gcloud ${GCPARGS} kms keys list \
    --location global \
    --keyring ${KEYRING} \
    --format='value(name)' \
    --filter "name~.*/${KEY}$" )
if [[ -z ${key} ]] ; then
  echo "Creating key: ${KEY}"
  gcloud ${GCPARGS} kms keys create ${KEY} \
    --location=global \
    --keyring=${KEYRING} \
    --purpose=encryption
fi

# Allow AppEngine service account to access key, if it doesn't already.
binding=$(
  gcloud ${GCPARGS} kms keys get-iam-policy ${KEY} \
    --location global \
    --keyring ${KEYRING} \
    | grep serviceAccount:${PROJECT}@appspot.gserviceaccount.com || : )
if [[ -z ${binding} ]]; then
  echo "Binding iam policy for accessing ${KEYRING}/${KEY}"
  gcloud ${GCPARGS} kms keys add-iam-policy-binding ${KEY} \
    --location=global \
    --keyring=${KEYRING} \
    --member=serviceAccount:${PROJECT}@appspot.gserviceaccount.com \
    --role=roles/cloudkms.cryptoKeyDecrypter
fi

which jwk-keygen &> /dev/null || \
    ( echo "Run: go get gopkg.in/square/go-jose.v2/jwk-keygen" && \
    exit 1 )

LOCATE_PRIVATE=jwk_sig_EdDSA_locate_${KEYID}
MONITORING_PRIVATE=jwk_sig_EdDSA_monitoring_${KEYID}

if [[ ! -f ${LOCATE_PRIVATE} ]] ; then
  # Create JWK key.
  echo "Creating private locate key: ${LOCATE_PRIVATE}"
  jwk-keygen --use=sig --alg=EdDSA --kid=locate_${KEYID}
fi

if [[ ! -f ${MONITORING_PRIVATE} ]] ; then
  # Create JWK key.
  echo "Creating private monitoring key: ${MONITORING_PRIVATE}"
  jwk-keygen --use=sig --alg=EdDSA --kid=monitoring_${KEYID}
fi

echo "Encrypting private locate signer key:"
ENC_SIGNER_KEY=$( cat ${LOCATE_PRIVATE} | gcloud ${GCPARGS} kms encrypt \
  --plaintext-file=- \
  --ciphertext-file=- \
  --location=global \
  --keyring=${KEYRING} \
  --key=${KEY} | base64 )

echo "Encrypting public monitoring verify key:"
ENC_VERIFY_KEY=$( cat ${MONITORING_PRIVATE}.pub | gcloud ${GCPARGS} kms encrypt \
  --plaintext-file=- \
  --ciphertext-file=- \
  --location=global \
  --keyring=${KEYRING} \
  --key=${KEY} | base64 )

echo ""
echo ""
echo "Include the following in app.yaml.${PROJECT}:"
echo ""
echo "env_variables:"
echo "  LOCATE_SIGNER_KEY: \"${ENC_SIGNER_KEY}\""
echo "  MONITORING_VERIFY_KEY: \"${ENC_VERIFY_KEY}\""
