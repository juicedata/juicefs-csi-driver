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

FROM python:3.8-slim-buster

ARG JFSCHAN

WORKDIR /app

ARG TARGETARCH
ARG JFS_PKG_URL=https://static.juicefs.com/release/bin_pkgs/latest_stable_full.tar.gz
ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_MOUNT_PATH=/usr/local/juicefs/mount/jfsmount
ENV JFSCHAN=${JFSCHAN}

RUN bash -c "if [[ '${TARGETARCH}' == amd64 ]]; then apt update && apt install -y software-properties-common wget gnupg gnupg2 && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    echo deb https://download.ceph.com/debian-16.2.15/ buster main | tee /etc/apt/sources.list.d/ceph.list && \
    apt-get update && apt-get install -y uuid-dev libglusterfs-dev glusterfs-common librados2 librados-dev; fi"

RUN apt-get update && apt-get install -y curl fuse procps iputils-ping strace iproute2 net-tools tcpdump lsof openssh-server openssh-client && \
    rm -rf /var/cache/apt/* && \
    mkdir -p /root/.juicefs /var/run/sshd && \
    ln -s /usr/local/bin/python /usr/bin/python && \
    mkdir /root/.acl && cp /etc/passwd /root/.acl/passwd && cp /etc/group /root/.acl/group && \
    ln -sf /root/.acl/passwd /etc/passwd && ln -sf /root/.acl/group  /etc/group

RUN jfs_mount_path=${JFS_MOUNT_PATH} && \
    bash -c "if [[ '${JFSCHAN}' == beta ]]; then curl -sSL https://static.juicefs.com/release/bin_pkgs/beta_full.tar.gz | tar -xz; jfs_mount_path=${JFS_MOUNT_PATH}.beta; \
    else curl -sSL ${JFS_PKG_URL} | tar -xz; fi;" && \
    bash -c "mkdir -p /usr/local/juicefs/mount; if [[ '${TARGETARCH}' == amd64 ]]; then cp Linux/mount.ceph $jfs_mount_path; else cp Linux/mount.aarch64 $jfs_mount_path; fi;" && \
    chmod +x ${jfs_mount_path} && cp juicefs.py ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI}

RUN /usr/bin/juicefs version

ENV K8S_VERSION v1.14.8
RUN curl -o /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/${TARGETARCH}/kubectl && chmod +x /usr/local/bin/kubectl
