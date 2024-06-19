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
imageName=$1
tag=${2:-latest}
platform=$3

REGIONS=(
	  registry.cn-hangzhou.aliyuncs.com
    registry.cn-chengdu.aliyuncs.com
    registry.cn-beijing.aliyuncs.com
    registry.cn-qingdao.aliyuncs.com
    registry.cn-shanghai.aliyuncs.com
    registry.cn-zhangjiakou.aliyuncs.com
    registry.cn-shenzhen.aliyuncs.com
    registry.cn-heyuan.aliyuncs.com
    registry.cn-guangzhou.aliyuncs.com
    registry.cn-wulanchabu.aliyuncs.com
    registry.cn-hongkong.aliyuncs.com
    registry.cn-huhehaote.aliyuncs.com
)

sync_image() {
  local registryName=$1
  local image=$2
  local platform=$3
  if [ "$platform" == "amd64" ]; then platform=""; fi
  local platform_suffix=${platform:+-$platform}

  echo "Syncing image: $image, platform: ${platform:-amd64}"

  if [ -n "$platform" ]; then
    docker pull $registryName/$image:${tag} --platform=${platform}
    for REGION in ${REGIONS[@]};
    do
      echo "in ${REGION}"
      docker login --username=${username} --password=${passwd} ${REGION}
      docker tag $registryName/$image:${tag} ${REGION}/juicedata/${image}:${tag}${platform_suffix}
      docker push ${REGION}/juicedata/${image}:${tag}${platform_suffix}
    done
  else
    docker pull $registryName/$image:${tag}
    for REGION in ${REGIONS[@]};
    do
      echo "in ${REGION}"
      docker login --username=${username} --password=${passwd} ${REGION}
      docker tag $registryName/$image:${tag} ${REGION}/juicedata/${image}:${tag}
      docker push ${REGION}/juicedata/${image}:${tag}
    done
  fi
}

if [ "$imageName" = "mount" ]; then
  if [ "$tag" = "latest" ]; then
    sync_image "juicedata" "mount"
    sync_image "juicedata" "juicefs-fuse"
  else
    sync_image "juicedata" "mount"
    sync_image "juicedata" "mount" "arm64"
    sync_image "juicedata" "juicefs-fuse"
    sync_image "juicedata" "juicefs-fuse" "arm64"
  fi
elif [ "$imageName" = "csi-driver" ]; then
  sync_image "juicedata" "juicefs-csi-driver"
  sync_image "juicedata" "juicefs-csi-driver" "arm64"
  sync_image "juicedata" "csi-dashboard"
  sync_image "juicedata" "csi-dashboard" "arm64"
else
  image=$(echo $imageName | rev | awk -F'/' '{print $1}' | rev)
  registryName=$(echo $imageName | awk -F'/' '{OFS="/"; $NF=""; NF--; print $0}')
  registryName=${registryName:-docker.io/library}
  sync_image $registryName $image $platform
fi
