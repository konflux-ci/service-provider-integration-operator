#!/bin/bash

NAME=${NAME:-spi-workload}

kubectl apply -f .tmp/${NAME}.yaml
