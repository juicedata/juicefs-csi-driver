#!/usr/bin/env bash
set +x

# This script is a reference to the fluid project's diagnostic script, thanks to the fluid project for the inspiration.

print_usage() {
  echo "Usage:"
  echo "    $0 COMMAND [OPTIONS]"
  echo "ENV:"
  echo "    JFS_NS: namespace of JuiceFS CSI Driver, default is kube-system."
  echo "    APP_NS: namespace of the application pod, default is default."
  echo "COMMAND:"
  echo "    help"
  echo "        Display this help message."
  echo "    get-mount"
  echo "        Print mount pod used by specified application pod."
  echo "    get-app"
  echo "        Print application pods using specified mount pod."
  echo "    collect"
  echo "        Collect csi & mount pods logs of juicefs."
  echo "OPTIONS:"
  echo "    -n, --namespace name"
  echo "        Namespace of application pod, this option takes percedence over the APP_NS environment variable, default is default."
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

DEFAULT_APP_NS="${APP_NS:-default}"
ORIGINAL_ARGS=( "$@" )

juicefs_resources() {
  local app=${pod}
  local namespace="${namespace:-$DEFAULT_APP_NS}"

  mkdir -p "$diagnose_dir/app"
  kubectl describe po "$app" -n $namespace &>"$diagnose_dir/app/$app-describe.log" 2>&1
  kubectl get po "$app" -oyaml -n $namespace &>"$diagnose_dir/app/$app.yaml" 2>&1

  NODE_NAME=$(kubectl -n ${namespace} get po ${app} -o jsonpath='{.spec.nodeName}')
  PVC_NAME=$(kubectl -n ${namespace} get po ${app} -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}')
  PV_NAME=$(kubectl -n ${namespace} get pvc $PVC_NAME -o jsonpath='{.spec.volumeName}')

  mkdir -p "$diagnose_dir/pv"
  kubectl get pv "$PV_NAME" -oyaml &>"$diagnose_dir/pv/pv-$PV_NAME.yaml" 2>&1
  kubectl describe pv "$PV_NAME" &>"$diagnose_dir/pv/pv-$PV_NAME-describe.log" 2>&1

  mkdir -p "$diagnose_dir/pvc"
  kubectl get pvc "$PVC_NAME" -n $namespace -oyaml &>"$diagnose_dir/pvc/$PVC_NAME.yaml" 2>&1
  kubectl describe pvc "$PVC_NAME" -n $namespace &>"$diagnose_dir/pvc/$PVC_NAME-describe.log" 2>&1
}

get_mount_pod() {
  if [ "${ORIGINAL_ARGS[1]}" == "" ]; then
    echo "EXAMPLES:"
    echo "    csi-doctor.sh get-mount $APP_POD_NAME"
    exit 1
  fi
  local app=${ORIGINAL_ARGS[1]}
  local namespace="${namespace:-$DEFAULT_APP_NS}"
  juicefs_namespace=${JFS_NS:-"kube-system"}

  set -e
  NODE_NAME=$(kubectl -n ${namespace} get po ${app} -o jsonpath='{.spec.nodeName}')
  PVC_NAME=$(kubectl -n ${namespace} get po ${app} -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}')
  PV_NAME=$(kubectl -n ${namespace} get pvc $PVC_NAME -o jsonpath='{.spec.volumeName}')
  PV_ID=$(kubectl get pv $PV_NAME -o jsonpath='{.spec.csi.volumeHandle}')
  MOUNT_POD_NAME=$(kubectl -n $juicefs_namespace get po --field-selector spec.nodeName=$NODE_NAME -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $PV_ID)
  printf "$juicefs_namespace\t$MOUNT_POD_NAME\n"
}

get_app_pod() {
  local mountpod=${mountpod}
  juicefs_namespace=${JFS_NS:-"kube-system"}

  pv_id=$(kubectl -n $juicefs_namespace get po $mountpod -o go-template='{{range $k,$v := .metadata.annotations}}{{if eq $k "juicefs-uniqueid"}}{{$v}}{{end}}{{end}}')
  pvs=$(kubectl get pv | awk '{print $1}' | grep -v NAME)
  for pv in $pvs; do
    volumeHandle=$(kubectl get pv $pv -oyaml |grep volumeHandle | awk '{print $2}')
    if [ "$pv_id" == "$volumeHandle" ]; then
      pvc_name=$(kubectl get pv $pv -o yaml | grep claimRef -A 6 | grep name: | awk '{print $2}')
      break
    fi
  done
  namespace=$(kubectl get pvc -A --field-selector=metadata.name=$pvc_name | grep -v NAME | awk '{print $1}')

  annos=$(kubectl -n $juicefs_namespace get po $mountpod -o go-template='{{range $k,$v := .metadata.annotations}}{{$v}}{{"\n"}}{{end}}')
  i=0
  pod_ids=()
  for anno in ${annos[@]}; do
    pod_id=$(echo $anno | grep -oP '(?<=pods/).+(?=/volumes)')
    if [[ $pod_id != "" ]]; then
      pod_ids+=($pod_id)
    fi
  done

  declare -A app_maps
  alls=$(kubectl get po -n $namespace | grep -v NAME | awk '{print $1}')
  for po in ${alls[@]}; do
    pod_id=$(kubectl -n $namespace get po $po -o jsonpath='{.metadata.uid}')
    app_maps[$pod_id]=$po
  done

  apps=()
  for pod_id in ${pod_ids[@]}; do
    app=${app_maps[$pod_id]}
    if [[ $app != "" ]]; then
      apps+=($app)
    fi
  done

  echo "Application pods using mount pod [$mountpod]:"
  echo "namespace:"
  echo "  $namespace"
  echo "apps: "
  for element in ${apps[@]}
  do
    echo "  $element"
  done
}

collect_mount_pod_msg() {
  local app=${pod}
  local namespace="${namespace:-$DEFAULT_APP_NS}"
  juicefs_namespace=${JFS_NS:-"kube-system"}
  mkdir -p "$diagnose_dir/juicefs-${juicefs_namespace}"

  NODE_NAME=$(kubectl -n ${namespace} get po ${app} -o jsonpath='{.spec.nodeName}')
  PVC_NAME=$(kubectl -n ${namespace} get po ${app} -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}')
  PV_NAME=$(kubectl -n ${namespace} get pvc $PVC_NAME -o jsonpath='{.spec.volumeName}')
  PV_ID=$(kubectl get pv $PV_NAME -o jsonpath='{.spec.csi.volumeHandle}')
  MOUNT_POD_NAME=$(kubectl -n $juicefs_namespace get po --field-selector spec.nodeName=$NODE_NAME -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $PV_ID)

  kubectl logs "$MOUNT_POD_NAME" -n $juicefs_namespace &>"$diagnose_dir/juicefs-$juicefs_namespace/mount-pod.log" 2>&1
  kubectl get po "$MOUNT_POD_NAME" -oyaml -n $juicefs_namespace &>"$diagnose_dir/juicefs-$juicefs_namespace/mount-pod.yaml" 2>&1
  kubectl describe po "$MOUNT_POD_NAME" -n $juicefs_namespace &>"$diagnose_dir/juicefs-$juicefs_namespace/mount-pod-describe.log" 2>&1

}

collect_juicefs_csi_msg() {
  local app=${pod}
  local namespace="${namespace:-$DEFAULT_APP_NS}"
  juicefs_namespace=${JFS_NS:-"kube-system"}
  mkdir -p "$diagnose_dir/juicefs-csi-node"
  mkdir -p "$diagnose_dir/juicefs-csi-controller"

  NODE_NAME=$(kubectl -n ${namespace} get po ${app} -o jsonpath='{.spec.nodeName}')

  CSI_NODE_POD_NAME=$(kubectl get po -n $juicefs_namespace --field-selector spec.nodeName=$NODE_NAME -l app=juicefs-csi-node -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
  kubectl logs "$CSI_NODE_POD_NAME" -c juicefs-plugin -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-node/$CSI_NODE_POD_NAME-juicefs-plugin.log" 2>&1
  kubectl get po "$CSI_NODE_POD_NAME" -oyaml -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-node/$CSI_NODE_POD_NAME.yaml" 2>&1
  kubectl describe po "$CSI_NODE_POD_NAME" -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-node/$CSI_NODE_POD_NAME-describe.log" 2>&1

  CSI_CTRL_POD_NAME="juicefs-csi-controller-0"
  kubectl logs "$CSI_CTRL_POD_NAME" -c juicefs-plugin -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-controller/$CSI_CTRL_POD_NAME-juicefs-plugin.log" 2>&1
  kubectl get po "$CSI_CTRL_POD_NAME" -oyaml -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-controller/$CSI_CTRL_POD_NAME.yaml" 2>&1
  kubectl describe po "$CSI_CTRL_POD_NAME" -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-controller/$CSI_CTRL_POD_NAME-describe.log" 2>&1
}

archive() {
  tar -zcvf "${current_dir}/diagnose_juicefs_${timestamp}.tar.gz" "${diagnose_dir}"
  echo "please get diagnose_juicefs_${timestamp}.tar.gz for diagnostics"
}

pd_collect() {
  echo "Start collecting, pod=${pod}, namespace=${namespace}"
  collect_mount_pod_msg
  collect_juicefs_csi_msg
  juicefs_resources
  archive
}

collect() {
  # ensure params
  pod_name=${pod:?"the name of pod using juicefs must be set"}
  namespace=${namespace:-"$DEFAULT_APP_NS"}

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
      get-mount|help)
        action=$1
        ;;
      get-app|help)
        action=$1
        ;;
      -n|--namespace)
        namespace=$2
        shift
        ;;
    esac
    shift
  done

  case ${action} in
    collect)
      collect
      ;;
    get-mount)
      get_mount_pod
      ;;
    get-app)
      get_app_pod
      ;;
    help)
      print_usage
      ;;
  esac
}

main "$@"
