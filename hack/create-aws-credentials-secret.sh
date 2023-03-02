#!/bin/sh

NAMESPACE=spi-system
SECRET_NAME=aws-secretsmanager-credentials

kubectl create secret generic ${SECRET_NAME} -n ${NAMESPACE} \
      --from-file=${HOME}/.aws/config \
      --from-file=${HOME}/.aws/credentials
