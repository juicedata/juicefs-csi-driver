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

FROM golang:1.19-buster AS builder

ARG GOPROXY
ARG TARGETARCH
ARG JUICEFS_REPO_URL=https://github.com/juicedata/juicefs
ARG JUICEFS_REPO_BRANCH=main
ARG JUICEFS_REPO_REF=${JUICEFS_REPO_BRANCH}

RUN bash -c "if [[ '${TARGETARCH}' == amd64 ]]; then mkdir -p /home/travis/.m2 && \
    wget -O /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb https://github.com/apple/foundationdb/releases/download/6.3.23/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    dpkg -i /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    wget -O - https://download.gluster.org/pub/gluster/glusterfs/10/rsa.pub | apt-key add - && \
    echo deb [arch=${TARGETARCH}] https://download.gluster.org/pub/gluster/glusterfs/10/LATEST/Debian/buster/${TARGETARCH}/apt buster main > /etc/apt/sources.list.d/gluster.list && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    echo deb https://download.ceph.com/debian-15.2.17/ buster main | tee /etc/apt/sources.list.d/ceph.list && \
    apt-get update && apt-get install -y uuid-dev libglusterfs-dev glusterfs-common librados2 librados-dev upx-ucl; fi"

WORKDIR /workspace
COPY --from=project **/*.go ./
COPY --from=project cmd ./cmd
COPY --from=project pkg ./pkg
COPY --from=project go.mod .
COPY --from=project go.sum .
COPY --from=project Makefile .
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
RUN apt-get update && apt-get install -y musl-tools
RUN make
# build juicefs
RUN cd /workspace && git clone --branch=$JUICEFS_REPO_BRANCH $JUICEFS_REPO_URL && \
    cd juicefs && git checkout $JUICEFS_REPO_REF && \
    bash -c "if [[ ${TARGETARCH} == amd64 ]]; then make juicefs.all && mv juicefs.all juicefs; else make juicefs; fi"

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

RUN apt update && \
    bash -c "if [[ ${TARGETARCH} == amd64 ]]; then apt install -y software-properties-common wget gnupg gnupg2 && mkdir -p /home/travis/.m2 && \
    wget -O /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb https://github.com/apple/foundationdb/releases/download/6.3.23/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    dpkg -i /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    wget -O - https://download.gluster.org/pub/gluster/glusterfs/10/rsa.pub | apt-key add - && \
    echo deb [arch=${TARGETARCH}] https://download.gluster.org/pub/gluster/glusterfs/10/LATEST/Debian/buster/${TARGETARCH}/apt buster main > /etc/apt/sources.list.d/gluster.list && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    echo deb https://download.ceph.com/debian-15.2.17/ buster main | tee /etc/apt/sources.list.d/ceph.list && \
    apt-get update && apt-get install -y uuid-dev libglusterfs-dev glusterfs-common librados2 librados-dev; fi"

RUN apt-get update && apt-get install -y curl fuse procps iputils-ping strace iproute2 net-tools tcpdump lsof && \
    rm -rf /var/cache/apt/* && mkdir -p /root/.juicefs && \
    ln -s /usr/local/bin/python /usr/bin/python && \
    mkdir /root/.acl && cp /etc/passwd /root/.acl/passwd && cp /etc/group /root/.acl/group && \
    ln -sf /root/.acl/passwd /etc/passwd && ln -sf /root/.acl/group  /etc/group

RUN jfs_mount_path=${JFS_MOUNT_PATH} && \
    bash -c "if [[ '${JFSCHAN}' == beta ]]; then curl -sSL https://static.juicefs.com/release/bin_pkgs/beta_full.tar.gz | tar -xz; jfs_mount_path=${JFS_MOUNT_PATH}.beta; \
    else curl -sSL https://static.juicefs.com/release/bin_pkgs/latest_stable_full.tar.gz | tar -xz; fi;" && \
    bash -c "mkdir -p /usr/local/juicefs/mount; if [[ '${TARGETARCH}' == amd64 ]]; then cp Linux/mount.ceph $jfs_mount_path; else cp Linux/mount.aarch64 $jfs_mount_path; fi;" && \
    chmod +x ${jfs_mount_path} && cp juicefs.py ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI}

COPY --from=builder /workspace/bin/juicefs-csi-driver /usr/local/bin/
COPY --from=builder /workspace/juicefs/juicefs /usr/local/bin/

RUN ln -s /usr/local/bin/juicefs /bin/mount.juicefs

RUN /usr/bin/juicefs version && /usr/local/bin/juicefs --version

ENTRYPOINT ["/tini", "-g", "--", "juicefs-csi-driver"]
