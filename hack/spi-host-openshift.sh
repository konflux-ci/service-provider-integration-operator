#!/bin/sh

NAMESPACE=spi-system
ROUTE_NAME=spi-oauth

OS_BASE_URL=$(oc get ingresses.config.openshift.io/cluster -o jsonpath={.spec.domain})

echo "${ROUTE_NAME}-${NAMESPACE}.${OS_BASE_URL}"
