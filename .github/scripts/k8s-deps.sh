#!/bin/bash
set -o errexit

SCRIPTS_DIR=$(cd $(dirname $0); pwd)
echo "SCRIPTS_DIR: $SCRIPTS_DIR"
cd "$SCRIPTS_DIR"

sudo apt-get install -y snapd curl netcat-openbsd
sudo snap install microk8s --classic
sudo microk8s start && sudo microk8s enable dns storage rbac
sudo microk8s.kubectl apply -f services.yaml

function wait_for_ready() {
    local redis_ip=''
    local minio_ip=''

    echo "Trying to get redis and minio pod IP ..."
    while true; do
        redis_ip=$(sudo microk8s.kubectl -n default get pods/redis-server-0 --output go-template='{{.status.podIP}}' || true)
        minio_ip=$(sudo microk8s.kubectl -n default get pods/minio-server-0 --output go-template='{{.status.podIP}}' || true)
        if [ -n "$redis_ip" -a -n "$minio_ip" ]; then
            echo "Redis IP: $redis_ip, MinIO IP: $minio_ip"
            break
        fi
        sleep 2
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

wait_for_ready
