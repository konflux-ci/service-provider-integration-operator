#!/bin/bash

#set -e

PROMETHEUS_HOST=${PROMETHEUS_HOST:-"prometheus.$(minikube ip).nip.io"}
GRAFANA_HOST=${GRAFANA_HOST:-"grafana.$(minikube ip).nip.io"}
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

echo 'Align time on podman VM'
podman machine ssh --username root date --set $(date -Iseconds)
echo 'Installing prometheus-operator'
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml \
        --force-conflicts=true \
        --server-side
echo
echo -n "Waiting deployment/prometheus-operator  become available: "
kubectl wait --for=condition=Available=True deployment/prometheus-operator -n default  --timeout=30s

echo
echo "Preparing Service account"
cat <<EOF | kubectl apply -n default -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
- apiGroups: [""]
  resources:
  - nodes
  - nodes/metrics
  - services
  - endpoints
  - pods
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources:
  - configmaps
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus
subjects:
- kind: ServiceAccount
  name: prometheus
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: spi-metrics-reader-prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: spi-metrics-reader
subjects:
- kind: ServiceAccount
  name: prometheus
  namespace: default
EOF

echo
echo 'Creating prometheus'
#kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/example/user-guides/getting-started/prometheus-pod-monitor.yaml
cat <<EOF | kubectl apply -n default -f -
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: prometheus
  labels:
    app: prometheus
spec:
  serviceAccountName: prometheus
  serviceMonitorNamespaceSelector: {}
  serviceMonitorSelector: {}
  podMonitorSelector: {}
  resources:
    requests:
      memory: 400Mi
EOF

kubectl wait --for=condition=Available=True prometheus/prometheus -n default  --timeout=30s
kubectl rollout status --watch --timeout=600s statefulset/prometheus-prometheus  -n default

echo
echo 'Creating ingress'
cat <<EOF | kubectl apply -n default -f -
kind: Ingress
apiVersion: networking.k8s.io/v1
metadata:
  name: prometheus-ingress
spec:
  rules:
  - host: ${PROMETHEUS_HOST}
    http:
      paths:
      - backend:
          service:
            name: prometheus-operated
            port:
              number: 9090
        path: "/"
        pathType: ImplementationSpecific
EOF
echo
echo 'Cleaning all ServiceMonitors'
kubectl delete  --all ServiceMonitor

echo 'Create a Prometheus ServiceMonitor'
cat <<EOF | kubectl apply -n default -f -
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: prometheus-self
  labels:
    name: prometheus
spec:
  selector:
    matchLabels:
      operated-prometheus: "true"
  namespaceSelector:
    any: true
  endpoints:
    - port: web
EOF

echo 'Create a SPI ServiceMonitor.'
cat <<EOF | kubectl apply -n default -f -
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  namespace: default
  name: spi-operator-service-monitor
  labels:
    prometheus: appstudio-sre
spec:
  endpoints:
    - path: /metrics
      port: metrics
      scheme: https
      bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
      tlsConfig:
          insecureSkipVerify: true
  namespaceSelector:
    matchNames:
      - spi-system
  selector:
    matchLabels:
      control-plane: "controller-manager"

EOF



echo 'Installing Grafana'
kustomize build "https://github.com/grafana-operator/grafana-operator/deploy/manifests?ref=v4.6.0" | kubectl apply -f -
echo
echo -n "Waiting deployment/grafana-operator-controller-manager become available: "
kubectl wait --for=condition=Available=True deployment/grafana-operator-controller-manager -n grafana-operator-system  --timeout=30s

echo
echo 'Creating Grafana instance'

cat <<EOF | kubectl apply -n grafana-operator-system -f -
apiVersion: integreatly.org/v1alpha1
kind: Grafana
metadata:
  name: spi-grafana
spec:
  client:
    preferService: true
  baseImage: docker.io/grafana/grafana:9.1.7
  ingress:
    enabled: True
    pathType: Prefix
    path: "/"
    hostname: ${GRAFANA_HOST}
  config:
    log:
      mode: "console"
      level: "error"
    log.frontend:
      enabled: true
    auth:
      disable_login_form: False
      disable_signout_menu: True
    auth.anonymous:
      enabled: True
  service:
    name: "grafana-service"
    labels:
      app: "grafana"
      type: "grafana-service"
  dashboardLabelSelector:
    - matchExpressions:
        - { key: app, operator: In, values: [grafana] }
  resources:
    # Optionally specify container resources
    limits:
      cpu: 200m
      memory: 200Mi
    requests:
      cpu: 100m
      memory: 100Mi
EOF

# until kubectl get pod -l name="myname" -o go-template='{{.items | len}}' | grep -qxF 1; do
#     echo "Waiting for pod"
#     sleep 1
# done
until kubectl get pod -l app="grafana" -o go-template='{{.items | len}}' -n grafana-operator-system  | grep -qxF 1; do
    echo "Waiting for Grafana pod"
    sleep 1
done


kubectl wait --for=condition=Available=True deployment/grafana-deployment -n grafana-operator-system  --timeout=30s


echo
PROM_INTERNAL_URL='http://'$(kubectl get endpoints/prometheus-operated -o json | jq -r '.subsets[0].addresses[0].ip')':9090'
echo 'Creating prometheus-appstudio-ds DS for Grafana. Connecting to:'${PROM_INTERNALIP}
cat <<EOF | kubectl apply -n grafana-operator-system -f -
apiVersion: integreatly.org/v1alpha1
kind: GrafanaDataSource
metadata:
  name: spi-prometheus-grafanadatasource
spec:
  name: middleware.yaml
  datasources:
    - name: prometheus-appstudio-ds
      type: prometheus
      access: proxy
      url: ${PROM_INTERNAL_URL}
      isDefault: true
      version: 1
      editable: true
      jsonData:
        tlsSkipVerify: true
        timeInterval: "30s"
EOF

kustomize build ${SCRIPT_DIR}/../config/monitoring/minikube | kubectl apply -f -

echo 'Creating Grafana dashboard Prometheus 2.0 Overview '
cat <<EOF | kubectl apply -n grafana-operator-system -f -
apiVersion: integreatly.org/v1alpha1
kind: GrafanaDashboard
metadata:
  name: prometheus-overview
  labels:
    app: grafana
spec:
  json:
    ""
  configMapRef:
    name: grafana-dashboard-prometheus-2-0-overview
    key: prometheus-2-0-overview_rev1.json
EOF


echo 'Creating Grafana dashboard: Controller Runtime Controllers Detail'
cat <<EOF | kubectl apply -n grafana-operator-system -f -
apiVersion: integreatly.org/v1alpha1
kind: GrafanaDashboard
metadata:
  name: controller-runtime
  labels:
    app: grafana
spec:
  json:
    ""
  configMapRef:
    name: grafana-dashboard-controller-runtime
    key: controller-runtime-controllers-detail_rev1.json
EOF

echo 'Creating Grafana dashboard: Go Processes'
cat <<EOF | kubectl apply -n grafana-operator-system -f -
apiVersion: integreatly.org/v1alpha1
kind: GrafanaDashboard
metadata:
  name: go-processes
  labels:
    app: grafana
spec:
  json:
    ""
  configMapRef:
    name: grafana-dashboard-go-processes
    key: go-processes_rev1.json
EOF

echo 'Creating Grafana dashboard: SPI Health'
cat <<EOF | kubectl apply -n grafana-operator-system -f -
apiVersion: integreatly.org/v1alpha1
kind: GrafanaDashboard
metadata:
  name: spi-health
  labels:
    app: grafana
spec:
  json:
    ""
  configMapRef:
    name: grafana-dashboard-spi-health
    key: spi-health.json
EOF

echo 'Creating Grafana dashboard: SPI Outbound Traffic'
cat <<EOF | kubectl apply -n grafana-operator-system -f -
apiVersion: integreatly.org/v1alpha1
kind: GrafanaDashboard
metadata:
  name: spi-outbound-traffic
  labels:
    app: grafana
spec:
  json:
    ""
  configMapRef:
    name: grafana-dashboard-spi-outbound-traffic
    key: spi-outbound-traffic.json
EOF

echo 'Creating Grafana dashboard: SPI SLO'
cat <<EOF | kubectl apply -n grafana-operator-system -f -
apiVersion: integreatly.org/v1alpha1
kind: GrafanaDashboard
metadata:
  name: spi-slo
  labels:
    app: grafana
spec:
  json:
    ""
  configMapRef:
    name: grafana-dashboard-spi-slo
    key: spi-slo.json
EOF

function decode() {
  case `uname` in
    Darwin)
      base64 -D
      ;;
    *)
      base64 -d
      ;;
  esac
}


echo
echo 'Prometheus url: https://'${PROMETHEUS_HOST}
echo 'Grafana url: https://'${GRAFANA_HOST}
echo 'Grafana admin user: '$(kubectl get secret/grafana-admin-credentials -n grafana-operator-system  --template={{.data.GF_SECURITY_ADMIN_USER}} | decode)
echo 'Grafana admin password: '$(kubectl get secret/grafana-admin-credentials -n grafana-operator-system  --template={{.data.GF_SECURITY_ADMIN_PASSWORD}} | decode)
