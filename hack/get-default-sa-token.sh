#!/bin/bash

SECRET_NAME=$(kubectl get serviceaccount default -n default -o jsonpath='{.secrets[0].name}')

kubectl get secret "${SECRET_NAME}" -o jsonpath='{.data.token}' | base64 -d
