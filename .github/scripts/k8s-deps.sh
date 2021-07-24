#!/bin/bash
set -o errexit

SCRIPTS_DIR=$(cd $(dirname $0); pwd)
echo "SCRIPTS_DIR: $SCRIPTS_DIR"
cd "$SCRIPTS_DIR"

KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v4.2.0/kustomize_v4.2.0_linux_amd64.tar.gz"

function die() {
    echo "$(date -Iseconds) [FATAL] $@"
    exit 128
}

function install_deps() {
    sudo apt-get install -y snapd curl netcat-openbsd bc dnsutils redis-tools
    curl -fsSL -o /tmp/kustomize.tar.gz "$KUSTOMIZE_URL" \
        && tar -xf /tmp/kustomize.tar.gz -C /usr/local/bin \
        && chmod a+x /usr/local/bin/kustomize \
        && kustomize version
    sudo snap install microk8s --classic
    sudo microk8s start && sudo microk8s enable dns storage rbac
}

function add_kube_resolv() {
    local kube_dns_ip=$(sudo microk8s.kubectl -n kube-system get svc/kube-dns -o 'go-template={{.spec.clusterIP}}')
    if [ -z "$kube_dns_ip" ]; then
        die "Could not get kube-dns IP."
    fi
    if ! grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' <<<"$kube_dns_ip" ; then
        die "Invalid kube-dns IP: $kube_dns_ip"
    fi
    mkdir -p /etc/systemd/resolved.conf.d/
    echo "Use kube-dns ${kube_dns_ip} to resolve .svc.cluster.local and .cluster.local"
    cat > /etc/systemd/resolved.conf.d/microk8s.conf <<EOF
[Resolve]
DNS=${kube_dns_ip}
#FallbackDNS=
Domains=~svc.cluster.local ~cluster.local
#LLMNR=no
#MulticastDNS=no
#DNSSEC=no
#Cache=yes
#DNSStubListener=yes
EOF
    sudo systemctl status systemd-resolved.service
    local resolved_ip=$(dig -4 kube-dns.kube-system.svc.cluster.local +short)
    if [ "x$resolved_ip" != "x$kube_dns_ip" ]; then
        die "Resolved kube-dns IP: ${resolved_ip} should equal kube-dns clusterIP: ${kube_dns_ip}"
    fi
}

function deploy_services() {
    sudo microk8s.kubectl apply -f services.yaml
}

function wait_for_ready() {
    local redis_ip=''
    local minio_ip=''

    echo "Trying to get redis and minio pod IP ..."
    local wait_seconds=2
    local max_wait_seconds=16
    while true; do
        redis_ip=$(sudo microk8s.kubectl -n default get pods/redis-server-0 --output go-template='{{.status.podIP}}' \
            | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' || true)
        minio_ip=$(sudo microk8s.kubectl -n default get pods/minio-server-0 --output go-template='{{.status.podIP}}' \
            | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' || true)
        if [ -n "$redis_ip" -a -n "$minio_ip" ]; then
            if [ -n "$redis_ip" -a -n "$minio_ip" ]; then
                echo "Redis IP: $redis_ip, MinIO IP: $minio_ip"
                break
            fi
        fi
        sleep $wait_seconds
        wait_seconds=$((wait_seconds * 2))
        if [ $wait_seconds -gt $max_wait_seconds ]; then
            wait_seconds=2
        fi
    done

    echo "Checking if Redis is OK ..."
    while true; do
        if nc -zvw 3 $redis_ip 6379 ; then
            break
        fi
        sleep 2
    done

    echo "Checking if MinIO is OK ..."
    while true; do
        if nc -zvw 3 $minio_ip 9000 ; then
            break
        fi
        sleep 2
    done
}

function main() {
    install_deps
    add_kube_resolv
    deploy_services
    wait_for_ready
}

