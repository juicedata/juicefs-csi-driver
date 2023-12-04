#!/bin/bash

function main() {
  deployMode=$1
  withoutKubelet=$2
  prepare_pkg
  echo "deployMode: " $deployMode
  echo "withoutKubelet: " $withoutKubelet
  if [ "${withoutKubelet}" == "withoutkubelet" ]; then
    deploy_csi_without_kubelet $deployMode
  elif [ $deployMode == "webhook" ]; then
    deploy_webhook
  elif [ $deployMode == "webhook-provisioner" ]; then
    deploy_webhook_provisioner
  else
    deploy_csi $deployMode
  fi
}

function prepare_pkg() {
  # ceph
  sudo apt install -y software-properties-common
  sudo wget -q -O- 'https://download.ceph.com/keys/release.asc' | sudo apt-key add -
  sudo apt-add-repository 'deb https://download.ceph.com/debian-pacific/ buster main'
  sudo apt-get update
  sudo apt-get install -y librados-dev libcephfs-dev librbd-dev

  # fdb
  sudo mkdir -p /home/travis/.m2
  sudo wget -O /home/travis/.m2/foundationdb-clients_6.3.23-1_amd64.deb https://github.com/apple/foundationdb/releases/download/6.3.23/foundationdb-clients_6.3.23-1_amd64.deb
  sudo dpkg -i /home/travis/.m2/foundationdb-clients_6.3.23-1_amd64.deb

  # gluster
  sudo wget -O - https://download.gluster.org/pub/gluster/glusterfs/10/rsa.pub | sudo apt-key add -
  sudo mkdir mkdir /etc/apt/sources.list.d/gluster.list
  sudo chmod 777 /etc/apt/sources.list.d/gluster.list
  sudo echo deb [arch=amd64] https://download.gluster.org/pub/gluster/glusterfs/10/LATEST/Debian/buster/amd64/apt buster main > /etc/apt/sources.list.d/gluster.list
  sudo apt-get update
  sudo apt-get install -y uuid-dev libglusterfs-dev glusterfs-common
}

function deploy_csi() {
  sudo microk8s.kubectl delete -f ${GITHUB_WORKSPACE}/deploy/webhook.yaml
  sudo microk8s.kubectl label ns default juicefs.com/enable-injection=false
  deployMode=$1
  sudo kustomize build ${GITHUB_WORKSPACE}/deploy/kubernetes/csi-ci/$deployMode | sed -e "s@juicedata/juicefs-csi-driver.*\$@juicedata/juicefs-csi-driver:${dev_tag}@g" \
    -e 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' -e "s@juicedata/csi-dashboard.*\$@juicedata/csi-dashboard:${dev_tag}@g" | sudo microk8s.kubectl apply -f -
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
    if [ $count = 4 ] && [ $all_count = 4 ]; then
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

function deploy_csi_without_kubelet() {
  sudo microk8s.kubectl delete -f ${GITHUB_WORKSPACE}/deploy/webhook.yaml
  sudo microk8s.kubectl label ns default juicefs.com/enable-injection=false
  deployMode=$1
  sudo kustomize build ${GITHUB_WORKSPACE}/deploy/kubernetes/csi-ci/without-kubelet/$deployMode | sed -e "s@juicedata/juicefs-csi-driver.*\$@juicedata/juicefs-csi-driver:${dev_tag}@g" \
    -e 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' -e "s@juicedata/csi-dashboard.*\$@juicedata/csi-dashboard:${dev_tag}@g" | sudo microk8s.kubectl apply -f -
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
    if [ $count = 4 ] && [ $all_count = 4 ]; then
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
  sudo ${GITHUB_WORKSPACE}/scripts/juicefs-csi-webhook-install.sh print | sed -e "s@juicedata/juicefs-csi-driver.*\$@juicedata/juicefs-csi-driver:${dev_tag}@g" \
    -e 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' -e 's@--v=5@--v=6@g' -e "s@juicedata/csi-dashboard.*\$@juicedata/csi-dashboard:${dev_tag}@g" | sudo microk8s.kubectl apply -f -
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
    if [ $count = 3 ] && [ $all_count = 2 ]; then
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
  sudo kustomize build ${GITHUB_WORKSPACE}/deploy/kubernetes/csi-ci/webhook-provisioner > ${GITHUB_WORKSPACE}/deploy/webhook.yaml
  sudo ${GITHUB_WORKSPACE}/hack/update_install_script.sh
  sudo ${GITHUB_WORKSPACE}/scripts/juicefs-csi-webhook-install.sh print | sed -e "s@juicedata/juicefs-csi-driver.*\$@juicedata/juicefs-csi-driver:${dev_tag}@g" \
    -e 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' -e 's@--v=5@--v=6@g' -e "s@juicedata/csi-dashboard.*\$@juicedata/csi-dashboard:${dev_tag}@g" | sudo microk8s.kubectl apply -f -
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
    if [ $count = 3 ] && [ $all_count = 2 ]; then
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
main $1 $2
