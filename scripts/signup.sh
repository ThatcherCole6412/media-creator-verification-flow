#!/bin/sh
set -eu

: "${CREATOR_EMAIL:?set CREATOR_EMAIL}"
: "${MEDIA_SOURCE:?set MEDIA_SOURCE}"

curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data "{\"email\":\"${CREATOR_EMAIL}\",\"source\":\"${MEDIA_SOURCE}\"}" \
  http://127.0.0.1:8080/signup
