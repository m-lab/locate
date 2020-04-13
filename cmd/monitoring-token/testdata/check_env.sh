#!/bin/bash
if [[ -z "${SERVICE_URL}" ]] ; then
    echo "SERVICE_URL is undefined"
    exit 1
fi
if [[ ${SERVICE_URL} != "$1" ]] ; then
    echo "SERVICE_URL: UNEXPECTED VALUE: $SERVICE_URL"
    exit 1
fi
