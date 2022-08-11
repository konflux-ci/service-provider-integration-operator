### To test SPI on KCP cluster

1. make sure you have kcp tooling installed locally and up to date
    - you can do it by cloning kcp repo and make install it -> `git clone https://github.com/kcp-dev/kcp && cd kcp && make install`
2. run cluster of your choice (e.g. minikube)
3. run `./hack/kcp-run.sh`
4. it should start after few seconds. Check that `.kcp` directory was created. You can find kcp log there `.kcp/kcp.log`. It should log something like `Discovering types for logical cluster "root:default:spi"` each 3 seconds
5. there is kubeconfig at `.kcp/.kcp/admin.kubeconfig`. The script sets it into `KUBECONFIG` env variable, so now `kubectl` calls are against kcp
6. check `kubectl kcp workspace`, it should output `Current workspace is "root:default:spi".`
7. the script created one workloadcluster `spi-workload-cluster`, you can check it with `kubectl get workloadclusters` and `kubectl get workloadclusters spi-workload-cluster -o yaml`. You should see status like this:
```
  status:
    conditions:
    - lastTransitionTime: "2022-06-23T11:56:05Z"
      message: No heartbeat yet seen
      reason: ErrorHeartbeat
      severity: Warning
      status: "False"
      type: Ready
    - lastTransitionTime: "2022-06-23T11:56:05Z"
      message: No heartbeat yet seen
      reason: ErrorHeartbeat
      severity: Warning
      status: "False"
      type: HeartbeatHealthy
    virtualWorkspaces:
    - url: https://192.168.0.89:6443/services/syncer/root:default:spi/spi-workload-cluster
```
This is fine, because physical cluster was not yet configured.

8. To setup physical cluster, you need to apply syncer deployment `kubectl apply -f .kcp/cluster_spi-workload-cluster_syncer.yaml`. You have to do this to physical cluster! So make sure you set KUBECONFIG right (open new terminal or something and run `kubectl config current-context`)
9. To verify all is working and connected, you should see:
    - lot of sync logging in kcp.log
    - weird namespaces in your physical cluster
  ```
~/dev/spi-operator (☸  minikube) [svpi137-kcpdeploy ✘]✭ λ kc get ns
NAME                                                              STATUS   AGE
default                                                           Active   2m25s
kcp0c4e3060002e46d2572520970d87f7446ce32e63046aa7e6f9ece4cc       Active   24s
kcpffa30c795dff741162ca736a394d52fd56fd1cd59b9387475b65245a       Active   25s
kcpsync6a762f74c5145c4be7573426b5aa245cfad58b46f3e77de8168a36d0   Active   32s
kube-node-lease                                                   Active   2m27s
kube-public                                                       Active   2m27s
kube-system                                                       Active   2m27s
```

- syncer pod running on physical cluster

```
~/dev/spi-operator (☸  minikube) [svpi137-kcpdeploy ✘]✭ λ kc get pods -n kcpsync6a762f74c5145c4be7573426b5aa245cfad58b46f3e77de8168a36d0
NAME                          READY   STATUS    RESTARTS   AGE
kcp-syncer-65c7cdf9d5-4cb4x   1/1     Running   0          68s
```
- if you check kcp workloadcluster again `kubectl get workloadclusters spi-workload-cluster -o yaml` Run with kcp kubeconfig! status must be ready and heartbeat healthy

10. you can run kustomize with kcp overlay against kcp `kubectl apply -k config/kcp`
11. To verify results
    - you must see `spi-system` namespace in kcp
    - namespace has label like `state.internal.workload.kcp.dev/spi-workload-cluster: Sync`
      - on physical cluster, there should be new `kcp<blablabla>` namespace
12. to kill everything, run `./hack/kcp-kill.sh`



## Links
 - onboarding doc for kcp service https://docs.google.com/document/d/1ygHQOBDMMnnsEDQoXSLSxaLUrplhOC42R3_kHCr25TI/edit#heading=h.mao1hpka76i1
   - in "How do I get access to this environment?" you skip points 1 and 2 because we already have appstudio organization, instead of that ask Matous or Alexey for invitation link. You will need to create new RH sso account, which is pain because it's later used to authenticate you on cmd. I'm using different user in browser for that. 