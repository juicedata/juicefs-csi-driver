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
registryName=$1
tag=$2

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
  local image=$1
  local platform=$2
  local platform_suffix=${platform:+-$platform}

  echo "Syncing image: $image, platform: ${platform:-default}"

  if [ -n "$platform" ]; then
    docker pull juicedata/$image:${tag} --platform=${platform}
    for REGION in ${REGIONS[@]};
    do
      echo ${REGION}
      docker tag juicedata/$image:${tag} ${REGION}/juicefs/${image}:${tag}${platform_suffix}
      docker push ${REGION}/juicefs/${image}:${tag}${platform_suffix}
    done
  else
    docker pull juicedata/$image:${tag}
    for REGION in ${REGIONS[@]};
    do
      echo ${REGION}
      docker tag juicedata/$image:${tag} ${REGION}/juicefs/${image}:${tag}
      docker push ${REGION}/juicefs/${image}:${tag}
    done
  fi
}

sync_sidecar_image() {
  local image=$1
  local platform=$2
  local platform_suffix=${platform:+-$platform}

  echo "Syncing image: $image, platform: ${platform:-default}"

  if [ -n "$platform" ]; then
    docker pull registry.k8s.io/sig-storage/$image:${tag} --platform=${platform}
    for REGION in ${REGIONS[@]};
    do
      echo ${REGION}
      docker tag registry.k8s.io/sig-storage/$image:${tag} ${REGION}/juicefs/${image}:${tag}${platform_suffix}
      docker push ${REGION}/juicefs/${image}:${tag}${platform_suffix}
    done
  else
    docker pull registry.k8s.io/sig-storage/$image:${tag}
    for REGION in ${REGIONS[@]};
    do
      echo ${REGION}
      docker tag registry.k8s.io/sig-storage$image:${tag} ${REGION}/juicefs/${image}:${tag}
      docker push ${REGION}/juicefs/${image}:${tag}
    done
  fi
}

docker login --username=${username} --password=${passwd} ${REGION}

if [ "$registryName" = "mount" ]; then
  if [ "$tag" = "latest" ]; then
    sync_image "mount"
    sync_image "juicefs-fuse"
  else
    sync_image "mount"
    sync_image "mount" "arm64"
    sync_image "juicefs-fuse"
    sync_image "juicefs-fuse" "arm64"
  fi
elif [ "$registryName" = "csi-driver" ]; then
  sync_image "juicefs-csi-driver"
  sync_image "juicefs-csi-driver" "arm64"
  sync_image "csi-dashboard"
  sync_image "csi-dashboard" "arm64"
elif [ "$registryName" = "livenessprobe" ]; then
  sync_image "livenessprobe"
  sync_image "livenessprobe" "arm64"
elif [ "$registryName" = "registrar" ]; then
  sync_image "csi-node-driver-registrar"
  sync_image "csi-node-driver-registrar" "arm64"
elif [ "$registryName" = "provisioner" ]; then
  sync_image "csi-provisioner"
  sync_image "csi-provisioner" "arm64"
elif [ "$registryName" = "resizer" ]; then
  sync_image "csi-resizer"
  sync_image "csi-resizer" "arm64"
else
  echo "Unknown registry name: $registryName"
  exit 1
fi
