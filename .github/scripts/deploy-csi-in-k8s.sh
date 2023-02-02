#!/bin/bash

function main() {
  deployMode=$1
  echo "deployMode: " $deployMode
  if [ $deployMode == "webhook" ]; then
    deploy_webhook
  elif [ $deployMode == "webhook-provisioner" ]; then
    deploy_webhook_provisioner
  else
    deploy_csi $deployMode
  fi
}

function deploy_csi() {
  sudo microk8s.kubectl delete -f ${GITHUB_WORKSPACE}/deploy/webhook.yaml
  sudo microk8s.kubectl label ns default juicefs.com/enable-injection=false
  deployMode=$1
  sudo kustomize build ${GITHUB_WORKSPACE}/deploy/kubernetes/csi-ci/$deployMode | sed -e "s@juicedata/juicefs-csi-driver.*\$@juicedata/juicefs-csi-driver:${dev_tag}@g" \
    -e 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' | sudo microk8s.kubectl apply -f -
  # Wait until the deploy finish
  timeout=0
  while true; do
    if [ $timeout -gt 60 ]; then
      echo "JuiceFS CSI is not ready within 5min."
      node_pod=$(sudo microk8s.kubectl -n kube-system get pods -o name | grep juicefs-csi-node | cut -d/ -f2)
      sudo microk8s.kubectl -n kube-system describe po $node_pod
      ctrl_pod=$(sudo microk8s.kubectl -n kube-system get pods -o name | grep juicefs-csi-controller | cut -d/ -f2)
      sudo microk8s.kubectl -n kube-system describe po $ctrl_pod
      exit 1
    fi
    timeout=$(expr $timeout + 1)
    echo "Wait JuiceFS CSI to be ready ..."
    # The juicefs-csi-{node|controller} pods' containers should be all ready
    all_count=$(sudo microk8s.kubectl -n kube-system get pods | grep juicefs-csi | wc -l)
    count=$(sudo microk8s.kubectl -n kube-system get pods | grep juicefs-csi | grep Running | awk '{print $2}' | tr '/' '-' | bc | grep '^0$' | wc -l)
    if [ $count = 2 ] && [ $all_count = 2 ]; then
      node_pod=$(sudo microk8s.kubectl -n kube-system get pods | grep Running | grep juicefs-csi-node | awk '{print $1}' | cut -d/ -f2)
      echo "JUICEFS_CSI_NODE_POD:" $node_pod
      echo "JUICEFS_CSI_NODE_POD=$node_pod" >>$GITHUB_ENV
      sudo microk8s.kubectl cp kube-system/$node_pod:/usr/local/bin/juicefs /usr/local/bin/juicefs -c juicefs-plugin &&
        sudo chmod a+x /usr/local/bin/juicefs && juicefs -V
      sudo microk8s.kubectl cp kube-system/$node_pod:/usr/bin/juicefs /usr/bin/juicefs -c juicefs-plugin &&
        sudo chmod a+x /usr/bin/juicefs && /usr/bin/juicefs version
      echo "JuiceFS CSI is ready."
      break
    fi
    sleep 5
  done
}

function deploy_webhook() {
  sudo microk8s.kubectl label ns default juicefs.com/enable-injection=true
  ds=$(sudo microk8s.kubectl get ds -n kube-system | grep juicefs-csi-node | wc -l)
  if [ $ds -gt 0 ]; then
    sudo microk8s.kubectl -n kube-system delete ds juicefs-csi-node
  fi
  sudo kustomize build ${GITHUB_WORKSPACE}/deploy/kubernetes/csi-ci/webhook >> ${GITHUB_WORKSPACE}/deploy/webhook.yaml
  sudo ${GITHUB_WORKSPACE}/hack/update_install_script.sh
  sudo ${GITHUB_WORKSPACE}/scripts/webhook.sh print | sed -e "s@juicedata/juicefs-csi-driver.*\$@juicedata/juicefs-csi-driver:${dev_tag}@g" \
    -e 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' -e 's@--v=5@--v=6@g' | sudo microk8s.kubectl apply -f -
  # Wait until the deploy finish
  timeout=0
  while true; do
    if [ $timeout -gt 60 ]; then
      echo "JuiceFS CSI is not ready within 5min."
      ctrl_pod=$(sudo microk8s.kubectl -n kube-system get pods -o name | grep juicefs-csi-controller | cut -d/ -f2)
      sudo microk8s.kubectl -n kube-system describe po $ctrl_pod
      sudo microk8s.kubectl -n kube-system sts juicefs-csi-controller
      exit 1
    fi
    timeout=$(expr $timeout + 1)
    echo "Wait JuiceFS CSI to be ready ..."
    # The juicefs-csi-controller pods' containers should be all ready
    all_count=$(sudo microk8s.kubectl -n kube-system get pods | grep juicefs-csi-controller | wc -l)
    count=$(sudo microk8s.kubectl -n kube-system get pods | grep juicefs-csi | grep Running | awk '{print $2}' | tr '/' '-' | bc | grep '^0$' | wc -l)
    if [ $count = 1 ] && [ $all_count = 1 ]; then
      ctrl_pod=juicefs-csi-controller-0
      sudo microk8s.kubectl cp kube-system/$ctrl_pod:/usr/local/bin/juicefs /usr/local/bin/juicefs -c juicefs-plugin &&
        sudo chmod a+x /usr/local/bin/juicefs && juicefs -V
      sudo microk8s.kubectl cp kube-system/$ctrl_pod:/usr/bin/juicefs /usr/bin/juicefs -c juicefs-plugin &&
        sudo chmod a+x /usr/bin/juicefs && /usr/bin/juicefs version
      echo "JuiceFS CSI is ready."
      break
    fi
    sleep 5
  done
}

function deploy_webhook_provisioner() {
  sudo microk8s.kubectl delete -f ${GITHUB_WORKSPACE}/deploy/webhook.yaml
  sudo microk8s.kubectl label ns default juicefs.com/enable-injection=true
  ds=$(sudo microk8s.kubectl get ds -n kube-system | grep juicefs-csi-node | wc -l)
  if [ $ds -gt 0 ]; then
    sudo microk8s.kubectl -n kube-system delete ds juicefs-csi-node
  fi
  sudo kustomize build ${GITHUB_WORKSPACE}/deploy/kubernetes/csi-ci/webhook-provisioner >> ${GITHUB_WORKSPACE}/deploy/webhook.yaml
  sudo ${GITHUB_WORKSPACE}/hack/update_install_script.sh
  sudo ${GITHUB_WORKSPACE}/scripts/webhook.sh print | sed -e "s@juicedata/juicefs-csi-driver.*\$@juicedata/juicefs-csi-driver:${dev_tag}@g" \
    -e 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' -e 's@--v=5@--v=6@g' | sudo microk8s.kubectl apply -f -
  # Wait until the deploy finish
  timeout=0
  while true; do
    if [ $timeout -gt 60 ]; then
      echo "JuiceFS CSI is not ready within 5min."
      ctrl_pod=$(sudo microk8s.kubectl -n kube-system get pods -o name | grep juicefs-csi-controller | cut -d/ -f2)
      sudo microk8s.kubectl -n kube-system describe po $ctrl_pod
      sudo microk8s.kubectl -n kube-system sts juicefs-csi-controller
      exit 1
    fi
    timeout=$(expr $timeout + 1)
    echo "Wait JuiceFS CSI to be ready ..."
    # The juicefs-csi-controller pods' containers should be all ready
    all_count=$(sudo microk8s.kubectl -n kube-system get pods | grep juicefs-csi-controller | wc -l)
    count=$(sudo microk8s.kubectl -n kube-system get pods | grep juicefs-csi | grep Running | awk '{print $2}' | tr '/' '-' | bc | grep '^0$' | wc -l)
    if [ $count = 1 ] && [ $all_count = 1 ]; then
      ctrl_pod=juicefs-csi-controller-0
      sudo microk8s.kubectl cp kube-system/$ctrl_pod:/usr/local/bin/juicefs /usr/local/bin/juicefs -c juicefs-plugin &&
        sudo chmod a+x /usr/local/bin/juicefs && juicefs -V
      sudo microk8s.kubectl cp kube-system/$ctrl_pod:/usr/bin/juicefs /usr/bin/juicefs -c juicefs-plugin &&
        sudo chmod a+x /usr/bin/juicefs && /usr/bin/juicefs version
      echo "JuiceFS CSI is ready."
      break
    fi
    sleep 5
  done
}
main $1
