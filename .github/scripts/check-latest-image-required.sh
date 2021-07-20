#!/bin/bash

function image_update_required() {
    local juicefs_latest_version=$(curl -fsSL https://api.github.com/repos/juicedata/juicefs/releases/latest | jq -r .tag_name)
    local juicefs_csi_latest_version=$(git describe --tags --match 'v*' | grep -oE 'v[0-9]+\.[1-9][0-9]*(\.[0-9]+)?')
    
    if [ -z "$juicefs_latest_version" ]; then
        echo "Cannot get the latest version of juicedata/juicefs." >&2
        exit 1
    fi
    if [ -z "$juicefs_csi_latest_version" ]; then
        echo "Cannot get the latest version of juicedata/juicefs-csi-driver."
        exit 1
    fi
    local juicefs_pub_at=$(curl -fsSL "https://api.github.com/repos/juicedata/juicefs/releases/tags/$juicefs_latest_version" | jq -r .published_at)
    local csi_pub_at=$(curl -fsSL "https://api.github.com/repos/juicedata/juicefs-csi-driver/releases/tags/$juicefs_csi_latest_version" | jq -r .published_at)
    local image_updated_at=$(curl -fsSL https://hub.docker.com/v2/repositories/juicedata/juicefs-csi-driver/tags/latest | jq -r .last_updated)

    if [ -z "$juicefs_pub_at" -o -z "$csi_pub_at" -o -z "$image_updated_at" ]; then
        return 0
    fi

    # Convert timestamp to unix seconds
    juicefs_pub_at=$(date -d "$juicefs_pub_at" +%s)
    csi_pub_at=$(date -d "$csi_pub_at" +%s)
    image_updated_at=$(date -d "$image_updated_at" +%s)

    if [ $image_updated_at -le $juicefs_pub_at -o $image_updated_at -le $csi_pub_at ]; then
        return 0
    fi
    return 1
}

function main() {
    if image_update_required; then
        echo yes
    else
        echo no
    fi
}

main
