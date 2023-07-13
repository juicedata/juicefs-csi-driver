# Copyright 2018 The Kubernetes Authors.
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

FROM golang:1.19-buster

ARG GOPROXY
ARG TARGETARCH
ARG JUICEFS_REPO_URL=https://github.com/juicedata/juicefs
ARG JUICEFS_REPO_BRANCH=main
ARG JUICEFS_REPO_REF=${JUICEFS_REPO_BRANCH}

RUN if [ ${TARGETARCH} -eq amd64 ]; then mkdir -p /home/travis/.m2 && \
    wget -O /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb https://github.com/apple/foundationdb/releases/download/6.3.23/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    dpkg -i /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    wget -O - https://download.gluster.org/pub/gluster/glusterfs/7/rsa.pub | apt-key add - && \
    echo deb [arch=${TARGETARCH}] https://download.gluster.org/pub/gluster/glusterfs/7/LATEST/Debian/buster/${TARGETARCH}/apt buster main > /etc/apt/sources.list.d/gluster.list && \
    apt-get update && apt-get install -y uuid-dev libglusterfs-dev glusterfs-common; fi

WORKDIR /workspace
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
RUN apt-get update && apt-get install -y musl-tools upx-ucl librados-dev libcephfs-dev librbd-dev && \
    cd /workspace && git clone --branch=$JUICEFS_REPO_BRANCH $JUICEFS_REPO_URL && \
    cd juicefs && git checkout $JUICEFS_REPO_REF && go get github.com/ceph/go-ceph@v0.4.0 && go mod tidy && \
    if [ ${TARGETARCH} -eq amd64 ]; then make juicefs.all && mv juicefs.all juicefs; else make juicefs.ceph && mv juicefs.ceph juicefs; fi && mv juicefs /usr/local/bin/juicefs

RUN ln -s /usr/local/bin/juicefs /bin/mount.juicefs && /usr/local/bin/juicefs --version
