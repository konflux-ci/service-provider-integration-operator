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
echo "Testing token injection"

SPI_URL=$(minikube service  service-provider-integration-api  --url -n vault)
echo $SPI_URL
curl -d '{"token":"githubtokenhere", "name":"some-service-token"}' -H "Content-Type: application/json" -X POST $SPI_URL/api/v1/token
kubectl create namespace usr-1 --dry-run=client -o yaml | kubectl apply -f -

# Создать несколько YAML-объектов из stdin
cat <<EOF | kubectl apply -n usr-1 -f -
apiVersion: appstudio.redhat.com/v1beta1
kind: AccessTokenSecret
metadata:
  name: ats
spec:
  accessTokenName: some-service-token
  target:
    secret:
        name: spi-data
        accessTokenKey: GITHUB_TOKEN
        labels:
          yiee: haa
EOF
sleep 3
INJECTED_VALUE=$(kubectl get secret spi-data -n usr-1 -o jsonpath='{.data.GITHUB_TOKEN}' | base64 -D)
if [ $INJECTED_VALUE = "githubtokenhere" ]; then
   echo "injected value "$INJECTED_VALUE
   exit 0
else
  echo "fail to inject value."
  exit 1
fi

