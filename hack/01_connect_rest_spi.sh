#!/bin/bash
#
# Copyright (C) 2021 Red Hat, Inc.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#         http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -e
echo "Connecting operator to SPI"

kubectl set env deployment/spi-controller-manager -n spi-system SPI_URL=http://service-provider-integration-api.vault
kubectl rollout status deployment/spi-controller-manager -n spi-system
while [ "$(kubectl get pods -l app.kubernetes.io/name=service-provider-integration-operator -n spi-system -o jsonpath='{.items[*].status.phase}')" != "Running" ]; do
   sleep 5
   echo "Waiting for service-provider-integration-operator  to be ready."
done

NEXT_WAIT_TIME=1
until [ $NEXT_WAIT_TIME -eq 5 ] || [ "$(kubectl logs -n spi-system deployment/spi-controller-manager -c manager | grep 'Starting workers')" ]; do
    sleep $NEXT_WAIT_TIME
    NEXT_WAIT_TIME=$(( NEXT_WAIT_TIME + 1 ))
done
[ $NEXT_WAIT_TIME -lt 5 ]

echo "operator connected to SPI"
