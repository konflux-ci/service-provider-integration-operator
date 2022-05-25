#!/bin/sh

export MINIKUBE_IP=$(minikube ip)

INGRESS=$(cat <<'END'
# This is an ingress to make Vault instance publicly reachable. This is not meant for any production setup, but rather
# strictly for development purposes. This makes running the operator locally easier by pointing it to the in-cluster
# Vault instance.
kind: Ingress
apiVersion: networking.k8s.io/v1
metadata:
  name: vault-ingress
spec:
  rules:
    - host: vault.${MINIKUBE_IP}.nip.io
      http:
        paths:
          - backend:
              service:
                name: spi-vault
                port:
                  name: http
            path: "/"
            pathType: ImplementationSpecific
END
)

TMPFILE=$(mktemp)

echo "$INGRESS" | envsubst > "$TMPFILE"

kubectl -n spi-system apply -f "$TMPFILE"

rm -f "$TMPFILE"
