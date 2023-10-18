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

FROM python:3.8-slim-bullseye

ARG JFSCHAN

WORKDIR /app

ARG TARGETARCH
ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_MOUNT_PATH=/usr/local/juicefs/mount/jfsmount
ENV JFSCHAN=${JFSCHAN}

RUN apt update && apt install -y software-properties-common wget gnupg gnupg2 && bash -c "if [[ '${TARGETARCH}' == amd64 ]]; then wget -O - https://download.gluster.org/pub/gluster/glusterfs/10/rsa.pub | apt-key add - && \
    echo deb [arch=${TARGETARCH}] https://download.gluster.org/pub/gluster/glusterfs/10/LATEST/Debian/bullseye/${TARGETARCH}/apt bullseye main > /etc/apt/sources.list.d/gluster.list && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    echo deb https://download.ceph.com/debian-17.2.6/ bullseye main | tee /etc/apt/sources.list.d/ceph.list && \
    apt-get update && apt-get install -y uuid-dev libglusterfs-dev glusterfs-common; fi"

RUN apt-get update && apt-get install -y librados2 curl fuse procps iputils-ping strace iproute2 net-tools tcpdump lsof librados-dev && \
    rm -rf /var/cache/apt/* && \
    bash -c "if [[ '${JFSCHAN}' == beta ]]; then curl -sSL https://juicefs.com/static/juicefs.py.beta -o ${JUICEFS_CLI}; else curl -sSL https://juicefs.com/static/juicefs -o ${JUICEFS_CLI}; fi; " && \
    chmod +x ${JUICEFS_CLI} && \
    mkdir -p /root/.juicefs && \
    ln -s /usr/local/bin/python /usr/bin/python && \
    mkdir /root/.acl && cp /etc/passwd /root/.acl/passwd && cp /etc/group /root/.acl/group && \
    ln -sf /root/.acl/passwd /etc/passwd && ln -sf /root/.acl/group  /etc/group

RUN bash -c "mkdir -p /usr/local/juicefs/mount && curl -sSL https://s.juicefs.com/static/Linux/mount.beta.ceph -o mount.ceph && if [[ '${JFSCHAN}' == beta ]]; then cp mount.ceph /usr/local/juicefs/mount/jfsmount.beta; else cp mount.ceph /usr/local/juicefs/mount/jfsmount; fi;"

