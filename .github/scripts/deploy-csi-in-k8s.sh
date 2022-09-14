#!/bin/bash

function main() {
  deployMode=$1
  echo "deployMode: " $deployMode
  sudo kustomize build ${GITHUB_WORKSPACE}/deploy/kubernetes/csi-ci/$deployMode | sed -e "s@juicedata/juicefs-csi-driver.*\$@juicedata/juicefs-csi-driver:${dev_tag}@g" \
                   -e 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' | sudo microk8s.kubectl apply -f -
  # Wait until the deploy finish
  timeout=0
  while true; do
    if [ $timeout -gt 60 ]; then
      echo "JuiceFS CSI is not ready within 5min."
      node_pod=$(sudo microk8s.kubectl -n default get pods -o name | grep juicefs-csi-node | cut -d/ -f2)
      sudo microk8s.kubectl -n default describe po $node_pod
      ctrl_pod=$(sudo microk8s.kubectl -n default get pods -o name | grep juicefs-csi-controller | cut -d/ -f2)
      sudo microk8s.kubectl -n default describe po $ctrl_pod
      exit 1
    fi
    timeout=$(expr $timeout + 1)
    echo "Wait JuiceFS CSI to be ready ..."
    # The juicefs-csi-{node|controller} pods' containers should be all ready
    all_count=$(sudo microk8s.kubectl -n default get pods | grep juicefs-csi | wc -l)
    count=$(sudo microk8s.kubectl -n default get pods | grep juicefs-csi | grep Running | awk '{print $2}' | tr '/' '-' | bc | grep '^0$' | wc -l)
    if [ $count = 2 ] && [ $all_count = 2 ]; then
      node_pod=$(sudo microk8s.kubectl -n default get pods | grep Running | grep juicefs-csi-node | awk '{print $1}' | cut -d/ -f2)
      echo "JUICEFS_CSI_NODE_POD:" $node_pod
      echo "JUICEFS_CSI_NODE_POD=$node_pod" >>$GITHUB_ENV
      sudo microk8s.kubectl cp default/$node_pod:/usr/local/bin/juicefs /usr/local/bin/juicefs -c juicefs-plugin && \
          sudo chmod a+x /usr/local/bin/juicefs && juicefs -V
      sudo microk8s.kubectl cp default/$node_pod:/usr/bin/juicefs /usr/bin/juicefs -c juicefs-plugin && \
          sudo chmod a+x /usr/bin/juicefs && /usr/bin/juicefs version
      echo "JuiceFS CSI is ready."
      break
    fi
    sleep 5
  done
}

main $1
