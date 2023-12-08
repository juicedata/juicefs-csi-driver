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
tag=${IMAGE_TAG}

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

for REGION in ${REGIONS[@]};
do
	echo ${REGION}
    docker login --username=${username} --password=${passwd} ${REGION}
    docker pull juicedata/juicefs-fuse:${tag}
	  docker tag juicedata/juicefs-fuse:${tag} ${REGION}/juicefs/juicefs-fuse:${tag}
    docker push ${REGION}/juicefs/juicefs-fuse:${tag}
done