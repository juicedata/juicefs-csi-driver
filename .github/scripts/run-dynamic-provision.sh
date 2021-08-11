#!/bin/bash

function deploy_dynamic_provision() {
  redis_db=$1
  secret_name=$(echo -n dynamic-provisioning | base64 -w 0)
  secret_metaurl=$(echo -n ${JUICEFS_REDIS_URL}/${redis_db} | base64 -w 0)
  secret_accesskey=$(echo -n ${JUICEFS_ACCESS_KEY} | base64 -w 0)
  secret_secretkey=$(echo -n ${JUICEFS_SECRET_KEY} | base64 -w 0)
  secret_storagename=$(echo -n ${JUICEFS_STORAGE} | base64 -w 0)
  secret_bucket=$(echo -n ${JUICEFS_BUCKET} | base64 -w 0)
  sed -i "s@juicefs-secret-name@${secret_name}@g" ${GITHUB_WORKSPACE}/.github/scripts/dynamic-provision-ce.yaml
  sed -i "s@juicefs-secret-metaurl@${secret_metaurl}@g" ${GITHUB_WORKSPACE}/.github/scripts/dynamic-provision-ce.yaml
  sed -i "s@juicefs-secret-access-key@${secret_accesskey}@g" ${GITHUB_WORKSPACE}/.github/scripts/dynamic-provision-ce.yaml
  sed -i "s@juicefs-secret-secret-key@${secret_secretkey}@g" ${GITHUB_WORKSPACE}/.github/scripts/dynamic-provision-ce.yaml
  sed -i "s@juicefs-secret-storagename@${secret_storagename}@g" ${GITHUB_WORKSPACE}/.github/scripts/dynamic-provision-ce.yaml
  sed -i "s@juicefs-secret-bucket@${secret_bucket}@g" ${GITHUB_WORKSPACE}/.github/scripts/dynamic-provision-ce.yaml

  echo "deploy storageclass & pvc & secret"
  sudo microk8s.kubectl create -f ${GITHUB_WORKSPACE}/.github/scripts/dynamic-provision-ce.yaml
}

function check_pod_success() {
  local pod_name=$1
  local timeout=0
  echo "Check app ${pod_name} is ready or not."
  while true; do
    if [ $timeout -gt 60 ]; then
      echo "pod/${pod_name} is not ready within 5min."
      sudo microk8s.kubectl -n default describe po ${pod_name}
      sudo microk8s.kubectl -n default describe pvc juicefs-pvc
      exit 1
    fi
    timeout=$(expr $timeout + 1)
    echo "Wait pod/${pod_name} to be ready ..."
    retval=$(sudo microk8s.kubectl -n default get pods | grep ${pod_name} | awk '{print $2}' | tr '/' '-' | bc | grep '^0$' || true)
    if [ x$retval = x0 ]; then
      echo "Pod ${pod_name} is ready."
      break
    fi
    sleep 5
  done
}

function check_pod_delete() {
  local pod_name=$1
  local timeout=0
  echo "Check app ${pod_name} is deleted or not."
  while true; do
    if [ $timeout -gt 60 ]; then
      echo "pod/${pod_name} is not deleted within 5min."
      sudo microk8s.kubectl -n default describe po ${pod_name}
      sudo microk8s.kubectl -n default describe pvc juicefs-pvc
      exit 1
    fi
    timeout=$(expr $timeout + 1)
    echo "Wait pod/${pod_name} to be deleted ..."
    retval=$(sudo microk8s.kubectl -n default get pods | grep ${pod_name} | awk '{print $1}')
    if [ x$retval = x ]; then
      echo "Pod ${pod_name} is deleted."
      break
    fi
    sleep 5
  done
}

function check_mount_point() {
  local redis_db=$1
  sudo juicefs mount -d "$JUICEFS_REDIS_URL/$redis_db" /jfs
  pv_count=$(ls /jfs | grep '^pvc-' | wc -l)
  if [ "x$pv_count" != x1 ]; then
    echo "Expected 1 PV, got $pv_count"
    exit 1
  fi
  pv_id=$(ls /jfs | grep '^pvc-')
  timeout=0
  while true; do
    if [ $timeout -gt 60 ]; then
      echo "/jfs/$pv_id/out.txt is not ready within 5min."
      exit 1
    fi
    timeout=$(expr $timeout + 1)
    echo "Wait /jfs/$pv_id/out.txt to be ready ..."
    if [ -e /jfs/$pv_id/out.txt ]; then
      break
    fi
    sleep 5
  done
  timeout=1
  while true; do
    if [ $timeout -gt 60 ]; then
      echo "content from /jfs/$pv_id/out.txt is null within 5min."
      exit 1
    fi
    timeout=$(expr $timeout + 1)
    echo "Wait content from /jfs/$pv_id/out.txt ..."
    dt1=$(tail -n 1 /jfs/$pv_id/out.txt)
    if [ -n "$dt1" ]; then
      break
    fi
    sleep 5
  done
  unix_ts1=$(date -d "$dt1" +%s)
  unix_ts2=$(date +%s)
  diff=$(echo "$unix_ts2-$unix_ts1" | bc)
  if [ "$diff" -lt 0 -o "$diff" -gt 15 ]; then
    echo "Unexpected time skew: $dt1, $(date -d@$unix_ts2 -u)"
    exit 1
  fi
}

function test_many_pods_in_one_pvc() {
  redis_db=$1
  for ((i = 1; i <= 3; i++)); do
    {
      echo "creat pod" app-${i}
      sed -i -e "s@app.*\$@app-${i}@g" ${GITHUB_WORKSPACE}/.github/scripts/app_use_dynamic_provision.yaml
      sudo microk8s.kubectl create -f ${GITHUB_WORKSPACE}/.github/scripts/app_use_dynamic_provision.yaml
      check_pod_success app-${i}
    }
  done
  check_mount_point ${redis_db}

  volume_name=$(sudo microk8s.kubectl get pvc juicefs-pvc -oyaml |grep volumeName: |awk '{print $2}')
  volume_id=$(sudo microk8s.kubectl get pv "${volume_name}" -oyaml |grep volumeHandle: | awk '{print $2}')
  node_name=$(sudo microk8s.kubectl get no | awk 'NR!=1' |sed -n '1p' |awk '{print $1}')
  mount_pod_name=juicefs-${node_name}-${volume_id}
  echo "Mount pod name: " ${mount_pod_name}
  echo "Check if mount pod is exist or not."
  retval=$(sudo microk8s.kubectl -n kube-system get pods | grep ${mount_pod_name} | awk '{print $1}')
  if [ x$retval = x ]; then
    echo "Can't find Pod ${mount_pod_name}."
    exit 1
  fi
  annotations_num=$(sudo microk8s.kubectl -n kube-system get po ${mount_pod_name} -oyaml  | sed -n '/annotations:/,/creationTimestamp:/p' |sed  '$d' |grep juicefs- |awk '{print $1}' |wc -l)
  if [ x$annotations_num = x3 ]; then
    echo "Pod ${mount_pod_name} has 3 juicefs- annotation."
  else
    echo "Pod ${mount_pod_name} has ${annotations_num} juicefs- annotation."
    exit 1
  fi
}

function test_delete_one() {
  echo "Check if it works well when delete one pod."
  sudo microk8s.kubectl -n default delete po app-1
  check_pod_delete app-1

  volume_name=$(sudo microk8s.kubectl get pvc juicefs-pvc -oyaml |grep volumeName: |awk '{print $2}')
  volume_id=$(sudo microk8s.kubectl get pv ${volume_name} -oyaml |grep volumeHandle: | awk '{print $2}')
  node_name=$(sudo microk8s.kubectl get no | awk 'NR!=1' |sed -n '1p' |awk '{print $1}')
  mount_pod_name=juicefs-${node_name}-${volume_id}
  echo "Mount pod name: " ${mount_pod_name}
  echo "Check if mount pod is exist or not."
  retval=$(sudo microk8s.kubectl -n kube-system get pods | grep ${mount_pod_name} | awk '{print $1}')
  if [ x$retval = x ]; then
    echo "Pod ${mount_pod_name} is deleted."
    exit 1
  fi
  annotations_num=$(sudo microk8s.kubectl -n kube-system get po ${mount_pod_name} -oyaml  | sed -n '/annotations:/,/creationTimestamp:/p' |sed  '$d' |grep juicefs- |awk '{print $1}' |wc -l)
  if [ x$annotations_num = x2 ]; then
    echo "Pod ${mount_pod_name} has 2 juicefs- annotation."
  else
    echo "Pod ${mount_pod_name} has ${annotations_num} juicefs- annotation."
    exit 1
  fi
}

function test_delete_all() {
  echo "Check if it works well when delete all pods."

  pods=$(sudo microk8s.kubectl -n default get po |grep app- |awk '{print $1}')
  for po in ${pods}; do
    {
      echo "delete pod" ${po}
      sudo microk8s.kubectl -n default delete po ${po}
      check_pod_delete ${po}
    }
  done

  volume_name=$(sudo microk8s.kubectl get pvc juicefs-pvc -oyaml |grep volumeName |awk '{print $2}')
  volume_id=$(sudo microk8s.kubectl get pv ${volume_name} -oyaml |grep volumeHandle | awk '{print $2}')
  node_name=$(sudo microk8s.kubectl get no | awk 'NR!=1' |sed -n '1p' |awk '{print $1}')
  mount_pod_name=juicefs-${node_name}-${volume_id}
  echo "Mount pod name: " ${mount_pod_name}
  echo "Check if mount pod is exist or not."
  retval=$(sudo microk8s.kubectl -n kube-system get pods | grep ${mount_pod_name} | awk '{print $1}' |wc -l)
  if [ x$retval != x0 ]; then
    echo "Pod ${mount_pod_name} is not deleted."
    exit 1
  fi
}

function main() {
  deploy_dynamic_provision 1
  test_many_pods_in_one_pvc 1
  test_delete_one
  test_delete_all
}

main
