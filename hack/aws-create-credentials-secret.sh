#!/bin/sh

kubectl create secret generic aws-secretsmanager-credentials \
  --from-file=${HOME}/.aws/config \
  --from-file=${HOME}/.aws/credentials \
  -n spi-system
