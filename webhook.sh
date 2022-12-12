#!/bin/bash

#
# Copyright 2022 Juicedata Inc
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

print_usage() {
  echo "Usage:"
  echo "    $0 COMMAND [OPTIONS]"
  echo "COMMAND:"
  echo "    help"
  echo "        Display this help message."
  echo "    install"
  echo "        Install juicefs csi driver in webhook mode."
  echo "    print"
  echo "        Print yamls of juicefs csi driver in webhook mode."
}

function gen_webhook_manifests() {
  need_cmd mktemp
  need_cmd openssl
  need_cmd curl

  K8S_SERVICE="juicefs-admission-webhook"
  K8S_NAMESPACE="kube-system"

  tmpdir=$(mktemp -d)

  ensure openssl genrsa -out ${tmpdir}/ca.key 2048 >/dev/null 2>&1
  ensure openssl req -x509 -new -nodes -key ${tmpdir}/ca.key -subj "/CN=${K8S_SERVICE}.${K8S_NAMESPACE}.svc" -days 1875 -out ${tmpdir}/ca.crt >/dev/null 2>&1
  ensure openssl genrsa -out ${tmpdir}/server.key 2048 >/dev/null 2>&1

  cat <<EOF >${tmpdir}/csr.conf
[req]
prompt = no
req_extensions = v3_req
distinguished_name = dn
[dn]
CN = ${K8S_SERVICE}.${K8S_NAMESPACE}.svc
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${K8S_SERVICE}
DNS.2 = ${K8S_SERVICE}.${K8S_NAMESPACE}
DNS.3 = ${K8S_SERVICE}.${K8S_NAMESPACE}.svc
EOF

  ensure openssl req -new -key ${tmpdir}/server.key -out ${tmpdir}/server.csr -config ${tmpdir}/csr.conf >/dev/null 2>&1
  ensure openssl x509 -req -in ${tmpdir}/server.csr -CA ${tmpdir}/ca.crt -CAkey ${tmpdir}/ca.key -CAcreateserial -out ${tmpdir}/server.crt -days 1875 -extensions v3_req -extfile ${tmpdir}/csr.conf >/dev/null 2>&1

  TLS_KEY=$(openssl base64 -A -in ${tmpdir}/server.key)
  TLS_CRT=$(openssl base64 -A -in ${tmpdir}/server.crt)
  CA_BUNDLE=$(openssl base64 -A -in ${tmpdir}/ca.crt)

  webhook_dir=$(pwd)
  cat ${webhook_dir}/webhook.yaml |
    sed -e "s/CA_BUNDLE/$CA_BUNDLE/g" \
      -e "s/TLS_KEY/$TLS_KEY/g" \
      -e "s/TLS_CRT/$TLS_CRT/g"
}

need_cmd() {
  if ! check_cmd "$1"; then
    err "need '$1' (command not found)"
  fi
}

check_cmd() {
  command -v "$1" >/dev/null 2>&1
}

ensure() {
  if ! "$@"; then err "command failed: $*"; fi
}

function main() {
  if [[ $# -eq 0 ]]; then
    print_usage
    exit 1
  fi

  action="help"

  while [[ $# -gt 0 ]]; do
    case $1 in
    -h | --help | "-?")
      print_usage
      exit 0
      ;;
    install | help)
      action=$1
      ;;
    print | help)
      action=$1
      ;;
    *)
      echo "Error: unsupported option $1" >&2
      print_usage
      exit 1
      ;;
    esac
    shift
  done

  case ${action} in
  install)
    gen_webhook_manifests | kubectl apply -f -
    ;;
  print)
    gen_webhook_manifests | cat
    ;;
  help)
    print_usage
    ;;
  esac
}

main $1
