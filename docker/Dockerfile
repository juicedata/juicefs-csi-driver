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

FROM golang:1.18-buster as builder

ARG GOPROXY
ARG JUICEFS_REPO_BRANCH=main
ARG JUICEFS_REPO_REF=${JUICEFS_REPO_BRANCH}
ARG JUICEFS_CSI_REPO_REF=master

RUN apt update && apt install -y software-properties-common && apt update && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    apt-add-repository 'deb https://download.ceph.com/debian-pacific/ buster main' && \
    apt update

WORKDIR /workspace
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
RUN apt-get update && apt-get install -y musl-tools upx-ucl librados-dev libcephfs-dev librbd-dev uuid-dev libglusterfs-dev && \
    git clone https://github.com/juicedata/juicefs-csi-driver && \
    cd juicefs-csi-driver && git checkout $JUICEFS_CSI_REPO_REF && make && \
    cd /workspace && git clone --branch=$JUICEFS_REPO_BRANCH https://github.com/juicedata/juicefs && \
    cd juicefs && git checkout $JUICEFS_REPO_REF && go get github.com/ceph/go-ceph@v0.4.0 && go mod tidy && \
    make juicefs.all && mv juicefs.all juicefs

FROM python:3.8-slim-buster

ARG TARGETARCH
ARG JFSCHAN
ARG JUICEFS_CE_MOUNT_IMAGE
ARG JUICEFS_EE_MOUNT_IMAGE

WORKDIR /app

ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_MOUNT_PATH=/usr/local/juicefs/mount/jfsmount
ENV JFSCHAN=${JFSCHAN}
ENV JUICEFS_CE_MOUNT_IMAGE=${JUICEFS_CE_MOUNT_IMAGE}
ENV JUICEFS_EE_MOUNT_IMAGE=${JUICEFS_EE_MOUNT_IMAGE}

ADD https://github.com/krallin/tini/releases/download/v0.19.0/tini-${TARGETARCH} /tini
RUN chmod +x /tini

RUN apt update && apt install -y software-properties-common wget gnupg gnupg2 && apt update && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    apt-add-repository 'deb https://download.ceph.com/debian-pacific/ buster main' && \
    apt update

RUN apt-get update && apt-get install -y librados2 librados-dev libcephfs-dev librbd-dev curl fuse procps iputils-ping strace iproute2 net-tools tcpdump lsof uuid-dev libglusterfs-dev && \
    rm -rf /var/cache/apt/* && \
    curl -sSL https://juicefs.com/static/juicefs -o ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI} && \
    mkdir -p /root/.juicefs && \
    ln -s /usr/local/bin/python /usr/bin/python && \
    mkdir /root/.acl && cp /etc/passwd /root/.acl/passwd && cp /etc/group /root/.acl/group && \
    ln -sf /root/.acl/passwd /etc/passwd && ln -sf /root/.acl/group  /etc/group

COPY --from=builder /workspace/juicefs-csi-driver/bin/juicefs-csi-driver /usr/local/bin/
COPY --from=builder /workspace/juicefs/juicefs /usr/local/bin/

RUN ln -s /usr/local/bin/juicefs /bin/mount.juicefs
COPY THIRD-PARTY /

RUN /usr/bin/juicefs version && /usr/local/bin/juicefs --version

ENTRYPOINT ["/tini", "--", "juicefs-csi-driver"]
