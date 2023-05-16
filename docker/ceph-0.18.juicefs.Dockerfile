#Copyright 2023 Juicedata Inc
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

FROM golang:1.18-buster

ARG GOPROXY
ARG JUICEFS_REPO_URL=https://github.com/juicedata/juicefs
ARG JUICEFS_REPO_BRANCH=main
ARG JUICEFS_REPO_REF=${JUICEFS_REPO_BRANCH}

RUN apt update && apt install -y software-properties-common && apt update && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    apt-add-repository 'deb https://download.ceph.com/debian-pacific/ buster main' && \
    apt update

WORKDIR /workspace
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
RUN apt-get update && apt-get install -y musl-tools upx-ucl librados-dev libcephfs-dev librbd-dev && \
    cd /workspace && git clone --branch=$JUICEFS_REPO_BRANCH $JUICEFS_REPO_URL && \
    cd juicefs && git checkout $JUICEFS_REPO_REF && make juicefs.ceph && mv juicefs.ceph juicefs && \
    mv juicefs /usr/local/bin/juicefs

RUN ln -s /usr/local/bin/juicefs /bin/mount.juicefs && /usr/local/bin/juicefs --version
