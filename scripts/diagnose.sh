#!/usr/bin/env bash
set +x

print_usage() {
  echo "Usage:"
  echo "    ./diagnose-juicefs.sh COMMAND [OPTIONS]"
  echo "COMMAND:"
  echo "    help"
  echo "        Display this help message."
  echo "    collect"
  echo "        Collect pods logs of juicefs."
  echo "OPTIONS:"
  echo "    -no, --node name"
  echo "        Set the name of node."
  echo "    -n, --namespace name"
  echo "        Set the namespace of juicefs csi driver."
}

run() {
  echo
  echo "-----------------run $*------------------"
  timeout 10s "$@"
  if [ $? != 0 ]; then
    echo "failed to collect info: $*"
  fi
  echo "------------End of ${1}----------------"
}

pod_status() {
  local namespace=${1:-"kube-system"}
  run kubectl get po -owide -n ${namespace} &>"$diagnose_dir/pods-${namespace}.log"
}

juicefs_resources() {
  mkdir -p "$diagnose_dir/pv"
  pvs=$(kubectl get pv | awk '{print $1}' | grep -v NAME)
  for pv in ${pvs}; do
    kubectl get pv "${pv}" -oyaml &>"$diagnose_dir/pv/pv-${pv}.yaml" 2>&1
    kubectl describe pv "${pv}" &>"$diagnose_dir/pv/pv-${pv}-describe.log" 2>&1
  done

  mkdir -p "$diagnose_dir/pvc"
  nss=$(kubectl get ns | awk '{print $1}' | grep -v NAME)
  for ns in ${nss}; do
    pvcs=$(kubectl get pvc -n ${ns} | awk '{print $1}' | grep -v NAME)
    if [ ${#pvcs} -ne 0 ]; then
      mkdir -p "$diagnose_dir/pvc/${ns}"
    fi
    for pvc in ${pvcs}; do
      mkdir -p "$diagnose_dir/pvc/${ns}/pvc-${pvc}"
      kubectl get pvc "${pvc}" -n ${ns} -oyaml &>"$diagnose_dir/pvc/${ns}/pvc-${pvc}/${pvc}.yaml" 2>&1
      kubectl describe pvc "${pvc}" -n ${ns} &>"$diagnose_dir/pvc/${ns}/pvc-${pvc}/${pvc}-describe.log" 2>&1
      pods=$(kubectl describe pvc "${pvc}" -n ${ns} | sed -n '/Used By: /,/Events:/p' | sed  '$d' | awk '{if (NR==1) print $3 ;else print $1}')
      for po in ${pods} ; do
        kubectl describe po "${po}" -n ${ns} &>"$diagnose_dir/pvc/${ns}/pvc-${pvc}/${po}.log" 2>&1
      done
    done
  done
}

core_juicefs_log() {
  local namespace="${juicefs_namespace}"
  local node_name="${node_name}"
  local pods
  mkdir -p "$diagnose_dir/juicefs-${namespace}"

  pods=$(kubectl get po -n ${namespace} -owide | grep "juicefs-${node_name}-" | awk '{print $1}')

  for po in ${pods}; do
    kubectl logs "${po}" -n ${namespace} &>"$diagnose_dir/juicefs-${namespace}/${po}.log" 2>&1
    kubectl get po "${po}" -oyaml -n ${namespace} &>"$diagnose_dir/juicefs-${namespace}/${po}.yaml" 2>&1
    kubectl describe po "${po}" -n ${namespace} &>"$diagnose_dir/juicefs-${namespace}/${po}-describe.log" 2>&1
  done
}

core_juicefs_csi_log() {
  local namespace="${juicefs_namespace}"
  local node_name="${node_name}"
  local pods
  mkdir -p "$diagnose_dir/juicefs-csi-${namespace}"
  pods=$(kubectl get po -n ${namespace} -owide | grep "${node_name}" | grep "juicefs-csi-node-" | awk '{print $1}' | grep -v NAME)
  for po in ${pods}; do
    kubectl logs "${po}" -c juicefs-plugin -n ${namespace} &>"$diagnose_dir/juicefs-csi-${namespace}/${po}-juicefs-plugin.log" 2>&1
    kubectl get po "${po}" -oyaml -n ${namespace} &>"$diagnose_dir/juicefs-csi-${namespace}/${po}.yaml" 2>&1
    kubectl describe po "${po}" -n ${namespace} &>"$diagnose_dir/juicefs-csi-${namespace}/${po}-describe.log" 2>&1
  done
}

archive() {
  tar -zcvf "${current_dir}/diagnose_juicefs_${timestamp}.tar.gz" "${diagnose_dir}"
  echo "please get diagnose_juicefs_${timestamp}.tar.gz for diagnostics"
}

pd_collect() {
  echo "Start collecting, node-name=${node_name}, juicefs-namespace=${juicefs_namespace}"
  pod_status "${juicefs_namespace}"
  core_juicefs_log
  core_juicefs_csi_log
  juicefs_resources
  archive
}

collect() {
  # ensure params
  node_name=${node_name:?"the name of node must be set"}
  juicefs_namespace=${juicefs_namespace:-"kube-system"}

  current_dir=$(pwd)
  timestamp=$(date +%s)
  diagnose_dir="/tmp/diagnose_juicefs_${timestamp}"
  mkdir -p "$diagnose_dir"

  pd_collect
}

main() {
  if [[ $# -eq 0 ]]; then
    print_usage
    exit 1
  fi

  action="help"

  while [[ $# -gt 0 ]]; do
    case $1 in
      -h|--help|"-?")
        print_usage
        exit 0;
        ;;
      collect|help)
        action=$1
        ;;
      -no|--node)
        node_name=$2
        shift
        ;;
      -n|--namespace)
        juicefs_namespace=$2
        shift
        ;;
      *)
        echo  "Error: unsupported option $1" >&2
        print_usage
        exit 1
        ;;
    esac
    shift
  done

  case ${action} in
    collect)
      collect
      ;;
    help)
      print_usage
      ;;
  esac
}

main "$@"