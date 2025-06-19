#!/bin/bash
#
# Copyright 2023 Juicedata Inc
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

#****************************************************************#
# ScriptName: sync.sh
#***************************************************************#
set -e

username=${ACR_USERNAME}
passwd=${ACR_TOKEN}
image_input=$1
platform=$2

REGIONS=(
	  registry.cn-hangzhou.aliyuncs.com
#    registry.cn-chengdu.aliyuncs.com
    registry.cn-beijing.aliyuncs.com
#    registry.cn-qingdao.aliyuncs.com
    registry.cn-shanghai.aliyuncs.com
#    registry.cn-zhangjiakou.aliyuncs.com
#    registry.cn-shenzhen.aliyuncs.com
#    registry.cn-heyuan.aliyuncs.com
    registry.cn-guangzhou.aliyuncs.com
    registry.cn-wulanchabu.aliyuncs.com
    registry.cn-hongkong.aliyuncs.com
#    registry.cn-huhehaote.aliyuncs.com
)

sync_image() {
  local registryName=$1
  local image=$2
  local tag=${3:-latest}
  local platform=${4:-"amd64"}
  local oldImage="${registryName}/${image}:${tag}"

  echo "Syncing image: $image"

  docker pull $oldImage --platform=${platform}
  for REGION in ${REGIONS[@]};
  do
    echo "in ${REGION}"
    local newImage="${REGION}/juicedata/${image}:${tag}"
    docker login --username=${username} --password=${passwd} ${REGION}
    docker tag $oldImage $newImage
    docker push $newImage
    sleep 10
  done
}

sync_multi_platform_image() {
    local registryName=$1
    local image=$2
    local tag=${3:-latest}
    oldImage="${registryName}/${image}:${tag}"
    archs=("amd64" "arm64")

    if [ -z "$oldImage" ]; then
        echo "old image is empty"
        return 1
    fi

    for REGION in ${REGIONS[@]};
    do
      echo "in ${REGION}"
      docker login --username="${username}" --password="${passwd}" "${REGION}"
      newImage="${REGION}/juicedata/${image}:${tag}"
      for arch in "${archs[@]}"; do
          arch_tag=$(echo "$arch" | sed 's/\//_/g')
          tagged_image="${newImage}-${arch_tag}"

          echo "Processing $arch: $oldImage => $tagged_image"

          docker pull --platform "$arch" "$oldImage"
          docker tag "$oldImage" "$tagged_image"
          docker push "$tagged_image"
      done

      docker manifest create ${newImage} ${newImage}-arm64 ${newImage}-amd64
      docker manifest push ${newImage}
      sleep 10
    done
}

parse_image_name() {
    local image="$1"

    local registry_name="docker.io"
    local image_name=""
    local tag="latest"

    if [[ "$image" =~ : ]]; then
        tag="${image##*:}"
        image="${image%:*}"
    fi

    if [[ "$image" =~ / ]]; then
        registry_name="${image%/*}"
        image_name="${image##*/}"
    else
        image_name="$image"
    fi

    if [[ "$registry_name" == "$image_name" ]]; then
        registry_name="docker.io"
    fi

    echo "$registry_name,$image_name,$tag"
}

function main () {
  result=$(parse_image_name "$image_input")
  IFS=',' read -r registryName image tag <<< "$result"
  echo "Registry Name: $registryName"
  echo "Image Name: $image"
  echo "Tag: $tag"
  if [ "$image" = "mount" ]; then
    if [[ $tag == *"latest"* || $tag == *"nightly"* || $tag == *"min"* || $tag == *"std"* ]]; then
      sync_image "juicedata" "mount" "$tag"
    else
      sync_multi_platform_image "juicedata" "mount" "$tag"
    fi
  elif [ "$image" = "juicefs-csi-driver" ]; then
    if [ "$tag" = "nightly" ]; then
      sync_image "juicedata" "juicefs-csi-driver" "$tag"
      sync_image "juicedata" "csi-dashboard" "$tag"
    else
      sync_multi_platform_image "juicedata" "juicefs-csi-driver" "$tag"
      sync_multi_platform_image "juicedata" "csi-dashboard" "$tag"
    fi
  elif [ "$image" = "juicefs-operator" ]; then
    if [ "$tag" = "nightly" ]; then
      sync_image "juicedata" "juicefs-operator" "$tag"
    else
      sync_multi_platform_image "juicedata" "juicefs-operator" "$tag"
    fi
  else
    if [ "$platform" == "all" ]; then
      sync_multi_platform_image "$registryName" "$image" "$tag"
    else
      sync_image "$registryName" "$image" "$tag" "$platform"
    fi
  fi
}

main
