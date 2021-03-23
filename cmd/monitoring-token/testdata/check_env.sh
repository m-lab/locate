#!/bin/bash
ENV_NAME=$1
ENV_VALUE=$2

if [[ -z "${!ENV_NAME}" ]] ; then
    echo "${ENV_NAME} is undefined"
    exit 1
fi
if [[ ${!ENV_NAME} != "${ENV_VALUE}" ]] ; then
    echo "${ENV_NAME}: UNEXPECTED VALUE: ${!ENV_NAME}"
    exit 1
fi
