### To test SPI on CPS/KCP cluster

1. make sure you have kcp tooling installed locally and up to date
    - you can do it by cloning kcp repo and make install it -> `git clone https://github.com/kcp-dev/kcp && cd kcp && make install`
2. On workload cluster `make deploy_vault_openshift`
3. On workload cluster `export VAULT_HOST=https://$(oc get route/spi-vault --namespace=spi-vault -o=jsonpath={'.spec.host'})`
4. On kcp `make deploy_vault_openshift`