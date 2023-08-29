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
  echo "    debug"
  echo "        Print various debug information for specified application pod."
  echo "    get-mount"
  echo "        Get mount pod used by specified application pod."
  echo "    get-oplog"
  echo "        Collect access log from mount pods that's being used by specified application pod."
  echo "    get-app"
  echo "        Get application pods using specified mount pod."
  echo "    collect"
  echo "        Collect logs for CSI Driver troubleshooting."
  echo "    exec"
  echo "        Execute command in all mount pods."
  echo "OPTIONS:"
  echo "    -n, --namespace NS"
  echo "        Namespace of application pod, this option takes percedence over the APP_NS environment variable, default is \"default\"."
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
kbctl=kubectl

SHOULD_CHECK_CSI_CONRTROLLER=''

debug_app_pod() {
  if [ "${ORIGINAL_ARGS[1]}" == "" ]; then
    echo "EXAMPLES:"
    echo "    csi-doctor.sh debug APP_POD_NAME --namespace NS"
    exit 1
  fi
  app=${ORIGINAL_ARGS[1]}
  local namespace="${namespace:-$DEFAULT_APP_NS}"
  juicefs_namespace=${JFS_NS:-"kube-system"}
  echo "## CSI Controller Image: $(${kbctl} -n $juicefs_namespace get po -l app=juicefs-csi-controller -o jsonpath='{.items[*].spec.containers[*].image}')"
  echo '## Application Pod Event'
  $kbctl -n $namespace get event --field-selector involvedObject.name=$app,type!=Normal
  PVC_NAMES=$(${kbctl} -n ${namespace} get po ${app} -o jsonpath='{..persistentVolumeClaim.claimName}')
  NODE_NAME=$(${kbctl} -n ${namespace} get po ${app} -o jsonpath='{.spec.nodeName}')
  app_pod_uid=$(${kbctl} -n $namespace get po $app -o jsonpath='{.metadata.uid}')
  for pvc_name in $PVC_NAMES
  do
    debug_pvc $pvc_name
    pv_name=$(${kbctl} -n ${namespace} get pvc $pvc_name -o jsonpath='{.spec.volumeName}')
    pv_id=$(${kbctl} get pv $pv_name -o jsonpath='{.spec.csi.volumeHandle}')
    if [ "$NODE_NAME" != "" ]; then
      mount_pod_names=$(${kbctl} -n $juicefs_namespace get po --field-selector spec.nodeName=$NODE_NAME -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $pv_id)
      for mount_pod_name in $mount_pod_names
      do
        annos=$(${kbctl} -n $juicefs_namespace get po $mount_pod_name -o go-template='{{range $k,$v := .metadata.annotations}}{{$v}}{{"\n"}}{{end}}')
        for anno in ${annos[@]}; do
          pod_uid=$(echo $anno | grep -oP '(?<=pods/).+(?=/volumes)')
          if [ "$pod_uid" == "$app_pod_uid" ]; then
            echo "## Mount Pod Image for $mount_pod_name: $(${kbctl} -n $juicefs_namespace get po $mount_pod_name -o jsonpath='{..containers[*].image}')"
            echo "## Mount Pod Event for $mount_pod_name"
            $kbctl get event -n $namespace --field-selector involvedObject.name=$mount_pod_name,type!=Normal
            echo "## Mount Pod Log: $mount_pod_name"
            $kbctl -n $juicefs_namespace logs $mount_pod_name --tail 1000 | grep -v "<INFO>" | grep -v "<DEBUG>" | tail -n 50
          fi
        done
      done
    fi
  done
  if [ "$SHOULD_CHECK_CSI_CONRTROLLER" == "true" ]; then
    echo "## CSI Controller Log"
    $kbctl -n $juicefs_namespace logs juicefs-csi-controller-0 --tail 20 -c juicefs-plugin
  fi
  if [ "$NODE_NAME" != "" ]; then
    CSI_NODE_POD_NAME=$(${kbctl} get po -n $juicefs_namespace --field-selector spec.nodeName=$NODE_NAME -l app=juicefs-csi-node -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
    if [ "$CSI_NODE_POD_NAME" != "" ]; then
      echo "## CSI Node Log: $CSI_NODE_POD_NAME"
      $kbctl -n $juicefs_namespace logs $CSI_NODE_POD_NAME -c juicefs-plugin --tail 20
    fi
  fi
}

debug_pvc() {
  pvc_name=$1
  pvc_phase=$(${kbctl} -n $namespace get pvc $pvc_name -ojsonpath={..phase})
  if [ "$pvc_phase" != "Bound" ]; then
    echo "## PVC Event: $pvc_name"
    SHOULD_CHECK_CSI_CONRTROLLER=true
    $kbctl get event -n $namespace --field-selector involvedObject.name=$pvc_name
  fi
}

get_mount_pod() {
  if [ "${ORIGINAL_ARGS[1]}" == "" ]; then
    echo "EXAMPLES:"
    echo "    csi-doctor.sh get-mount APP_POD_NAME"
    exit 1
  fi
  app=${ORIGINAL_ARGS[1]}
  local namespace="${namespace:-$DEFAULT_APP_NS}"
  juicefs_namespace=${JFS_NS:-"kube-system"}

  set -e
  NODE_NAME=$(${kbctl} -n ${namespace} get po ${app} -o jsonpath='{.spec.nodeName}')
  PVC_NAMES=$(${kbctl} -n ${namespace} get po ${app} -o jsonpath='{..persistentVolumeClaim.claimName}')
  PV_NAME=$(${kbctl} -n ${namespace} get pvc $PVC_NAME -o jsonpath='{.spec.volumeName}')

  for pvc_name in $PVC_NAMES
  do
    pv_name=$(${kbctl} -n ${namespace} get pvc $pvc_name -o jsonpath='{.spec.volumeName}')
    pv_id=$(${kbctl} get pv $pv_name -o jsonpath='{.spec.csi.volumeHandle}')
    if [ "$NODE_NAME" != "" ]; then
      mount_pod_names=$(${kbctl} -n $juicefs_namespace get po --field-selector spec.nodeName=$NODE_NAME -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $pv_id)
      for mount_pod_name in $mount_pod_names
      do
        printf "$juicefs_namespace\t$mount_pod_name\n"
      done
    fi
  done
}

get_oplog() {
  if [ "${ORIGINAL_ARGS[1]}" == "" ]; then
    echo "EXAMPLES:"
    echo "    csi-doctor.sh get-oplog APP_POD_NAME"
    exit 1
  fi
  app=${ORIGINAL_ARGS[1]}
  local namespace="${namespace:-$DEFAULT_APP_NS}"
  juicefs_namespace=${JFS_NS:-"kube-system"}

  set -e
  NODE_NAME=$(${kbctl} -n ${namespace} get po ${app} -o jsonpath='{.spec.nodeName}')
  PVC_NAMES=$(${kbctl} -n ${namespace} get po ${app} -o jsonpath='{..persistentVolumeClaim.claimName}')
  PV_NAME=$(${kbctl} -n ${namespace} get pvc $PVC_NAME -o jsonpath='{.spec.volumeName}')

  for pvc_name in $PVC_NAMES
  do
    pv_name=$(${kbctl} -n ${namespace} get pvc $pvc_name -o jsonpath='{.spec.volumeName}')
    pv_id=$(${kbctl} get pv $pv_name -o jsonpath='{.spec.csi.volumeHandle}')
    if [ "$NODE_NAME" != "" ]; then
      mount_pod_names=$(${kbctl} -n $juicefs_namespace get po --field-selector spec.nodeName=$NODE_NAME -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $pv_id)
      for mount_pod_name in $mount_pod_names
      do
        mp=$(${kbctl} -n ${juicefs_namespace} get po $mount_pod_name -ojsonpath={..preStop.exec.command[-1]} | grep -oP '\S+$')
        echo ${kbctl} -n ${juicefs_namespace} exec -it -c jfs-mount $mount_pod_name -- cat ${mp}/.accesslog
        echo ${kbctl} -n ${juicefs_namespace} exec -it -c jfs-mount $mount_pod_name -- cat ${mp}/.ophistory
      done
    fi
  done
}

get_app_pod() {
  if [ "${ORIGINAL_ARGS[1]}" == "" ]; then
    echo "EXAMPLES:"
    echo "    csi-doctor.sh get-app MOUNT_POD_NAME"
    exit 1
  fi
  local mountpod=${ORIGINAL_ARGS[1]}
  juicefs_namespace=${JFS_NS:-"kube-system"}

  set -e
  pv_id=$(${kbctl} -n $juicefs_namespace get po $mountpod -o go-template='{{range $k,$v := .metadata.annotations}}{{if eq $k "juicefs-uniqueid"}}{{$v}}{{end}}{{end}}')
  pvs=$(${kbctl} get pv --no-headers | awk '{print $1}')
  for pv in $pvs; do
    volumeHandle=$(${kbctl} get pv $pv -ojsonpath={..volumeHandle})
    if [ "$pv_id" == "$volumeHandle" ]; then
      pvc_name=$(${kbctl} get pv $pv -ojsonpath={..claimRef.name})
      break
    fi
  done
  namespace=$(${kbctl} get pvc -A --field-selector=metadata.name=$pvc_name -ojsonpath={..namespace})

  annos=$(${kbctl} -n $juicefs_namespace get po $mountpod -o go-template='{{range $k,$v := .metadata.annotations}}{{$v}}{{"\n"}}{{end}}')
  i=0
  pod_ids=()
  set +e
  for anno in ${annos[@]}; do
    pod_id=$(echo $anno | grep -oP '(?<=pods/).+(?=/volumes)')
    if [[ $pod_id != "" ]]; then
      pod_ids+=($pod_id)
    fi
  done
  set -e

  declare -A app_maps
  alls=$(${kbctl} get po -n $namespace --no-headers | awk '{print $1}')
  for po in ${alls[@]}; do
    pod_id=$(${kbctl} -n $namespace get po $po -o jsonpath='{.metadata.uid}')
    app_maps[$pod_id]=$po
  done

  apps=()
  for pod_id in ${pod_ids[@]}; do
    app=${app_maps[$pod_id]}
    if [[ $app != "" ]]; then
      apps+=($app)
    fi
  done

  for element in ${apps[@]}
  do
    printf "$namespace\t$element\n"
  done
}

mount_exec() {
  cmd=${ORIGINAL_ARGS[@]:1}
  if [ "${cmd}" == "" ]; then
    echo "EXAMPLES:"
    echo "    csi-doctor.sh exec -- grep -nr master /root/.juicefs"
    exit 1
  fi
  juicefs_namespace=${JFS_NS:-"kube-system"}
  mount_pods=$(${kbctl} get pods -n $juicefs_namespace -l app.kubernetes.io/name=juicefs-mount --no-headers -o custom-columns=":metadata.name")
  set -x
  for mount_pod in $mount_pods
  do
    $kbctl -n $juicefs_namespace exec -it $mount_pod $cmd
  done
  set +x
}

collect_pv() {
  local namespace="${namespace:-$DEFAULT_APP_NS}"
  juicefs_namespace=${JFS_NS:-"kube-system"}
  mkdir -p "$diagnose_dir/juicefs-${juicefs_namespace}"

  NODE_NAME=$(${kbctl} -n ${namespace} get po ${app} -o jsonpath='{.spec.nodeName}')
  PVC_NAMES=$(${kbctl} -n ${namespace} get po ${app} -o jsonpath='{..persistentVolumeClaim.claimName}')
  mkdir -p "$diagnose_dir/pv"
  mkdir -p "$diagnose_dir/pvc"
  for pvc_name in $PVC_NAMES
  do

    $kbctl get pvc "$pvc_name" -n $namespace -oyaml &>"$diagnose_dir/pvc/$pvc_name.yaml" 2>&1
    $kbctl describe pvc "$pvc_name" -n $namespace &>"$diagnose_dir/pvc/$pvc_name-describe.log" 2>&1

    pv_name=$(${kbctl} -n ${namespace} get pvc $pvc_name -o jsonpath='{.spec.volumeName}')

    $kbctl get pv "$pv_name" -oyaml &>"$diagnose_dir/pv/pv-$pv_name.yaml" 2>&1
    $kbctl describe pv "$pv_name" &>"$diagnose_dir/pv/pv-$pv_name-describe.log" 2>&1

    pv_id=$(${kbctl} get pv $pv_name -o jsonpath='{.spec.csi.volumeHandle}')
    if [ "$NODE_NAME" != "" ]; then
      mount_pod_names=$(${kbctl} -n $juicefs_namespace get po --field-selector spec.nodeName=$NODE_NAME -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $pv_id)
      for mount_pod_name in $mount_pod_names
      do
        $kbctl get po "$mount_pod_name" -oyaml -n $juicefs_namespace &>"$diagnose_dir/juicefs-$juicefs_namespace/mount-pod-$mount_pod_name.yaml" 2>&1
        $kbctl describe po "$mount_pod_name" -n $juicefs_namespace &>"$diagnose_dir/juicefs-$juicefs_namespace/mount-pod-$mount_pod_name.log" 2>&1
        $kbctl logs "$mount_pod_name" -n $juicefs_namespace --all-containers=true >>"$diagnose_dir/juicefs-$juicefs_namespace/mount-pod-$mount_pod_name.log" 2>&1
      done
    fi
  done

}

collect_juicefs_csi_msg() {
  local namespace="${namespace:-$DEFAULT_APP_NS}"
  juicefs_namespace=${JFS_NS:-"kube-system"}
  mkdir -p "$diagnose_dir/juicefs-csi-node"
  mkdir -p "$diagnose_dir/juicefs-csi-controller"

  NODE_NAME=$(${kbctl} -n ${namespace} get po ${app} -o jsonpath='{.spec.nodeName}')

  CSI_NODE_POD_NAME=$(${kbctl} get po -n $juicefs_namespace --field-selector spec.nodeName=$NODE_NAME -l app=juicefs-csi-node -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
  $kbctl logs "$CSI_NODE_POD_NAME" -c juicefs-plugin -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-node/$CSI_NODE_POD_NAME-juicefs-plugin.log" 2>&1
  $kbctl get po "$CSI_NODE_POD_NAME" -oyaml -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-node/$CSI_NODE_POD_NAME.yaml" 2>&1
  $kbctl describe po "$CSI_NODE_POD_NAME" -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-node/$CSI_NODE_POD_NAME-describe.log" 2>&1

  CSI_CTRL_POD_NAME="juicefs-csi-controller-0"
  $kbctl logs "$CSI_CTRL_POD_NAME" -c juicefs-plugin -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-controller/$CSI_CTRL_POD_NAME-juicefs-plugin.log" 2>&1
  $kbctl get po "$CSI_CTRL_POD_NAME" -oyaml -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-controller/$CSI_CTRL_POD_NAME.yaml" 2>&1
  $kbctl describe po "$CSI_CTRL_POD_NAME" -n $juicefs_namespace &>"$diagnose_dir/juicefs-csi-controller/$CSI_CTRL_POD_NAME-describe.log" 2>&1
}

pd_collect() {
  mkdir -p "$diagnose_dir/app"
  $kbctl get po "$app" -oyaml -n $namespace &>"$diagnose_dir/app/$app.yaml" 2>&1
  code=$?
  if [ "${code}" != "0" ]; then
    echo "$namespace/$app not found, abort"
    rm -rf $diagnose_dir
    exit $code
  fi
  echo "Start collecting logs for ${namespace}/${app}"
  $kbctl describe po "$app" -n $namespace &>"$diagnose_dir/app/$app-describe.log" 2>&1
  collect_pv
  collect_juicefs_csi_msg
  set -e
  archive_file_path="${current_dir}/diagnose_juicefs_${timestamp}.tar.gz"
  tar -zcf "${archive_file_path}" -C /tmp $(basename $diagnose_dir)
  echo "Results have been saved to ${archive_file_path}"
}

collect() {
  if [ "${ORIGINAL_ARGS[1]}" == "" ]; then
    echo "EXAMPLES:"
    echo "    csi-doctor.sh collect APP_POD_NAME"
    exit 1
  fi
  app=${ORIGINAL_ARGS[1]}
  namespace=${namespace:-"$DEFAULT_APP_NS"}
  juicefs_namespace=${juicefs_namespace:-"kube-system"}
  current_dir=$(pwd)
  timestamp=$(date +%s)
  diagnose_result="${app}.diagnose.tar.gz"
  diagnose_dir="/tmp/${app}.diagnose"
  mkdir -p "$diagnose_dir"

  pd_collect
  rm -rf $diagnose_dir
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
      debug|help)
        action=$1
        ;;
      collect|help)
        action=$1
        ;;
      get-mount|help)
        action=$1
        ;;
      get-oplog|help)
        action=$1
        ;;
      get-app|help)
        action=$1
        ;;
      exec|help)
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
    debug)
      debug_app_pod
      ;;
    collect)
      collect
      ;;
    get-mount)
      get_mount_pod
      ;;
    get-oplog)
      get_oplog
      ;;
    get-app)
      get_app_pod
      ;;
    exec)
      mount_exec
      ;;
    help)
      print_usage
      ;;
  esac
}

main "$@"
