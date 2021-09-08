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

ARG BASE_IMAGE
FROM ${BASE_IMAGE} as builder

ARG GO_ARCH=amd64
ARG GOROOT=/usr/local/go
ARG GOPROXY
ARG JUICEFS_REPO_BRANCH=main
ARG JUICEFS_REPO_TAG
ARG JUICEFS_CSI_REPO_REF=master

RUN mkdir -p ${GOROOT} && \
    curl -fsSL https://golang.org/dl/go1.14.linux-${GO_ARCH}.tar.gz | \
    tar -xzf - -C ${GOROOT} --strip-components=1 && \
    ${GOROOT}/bin/go version && ${GOROOT}/bin/go env && \
    yum -y install libcephfs-devel librados-devel librbd-devel gcc make git upx

ENV GOROOT=${GOROOT} \
    GOPATH=/go \
    GOPROXY=${GOPROXY:-https://proxy.golang.org,direct} \
    CGO_ENABLED=1 \
    PATH="${GOROOT}/bin:${GOPATH}/bin:${PATH}"

WORKDIR /juicefs-csi-driver
COPY . .
RUN make

WORKDIR /workspace
RUN git clone https://github.com/juicedata/juicefs-csi-driver && \
    cd juicefs-csi-driver && git checkout $JUICEFS_CSI_REPO_REF && make && \
    cd /workspace && git clone --branch=$JUICEFS_REPO_BRANCH https://github.com/juicedata/juicefs && \
    cd juicefs && git checkout $JUICEFS_REPO_TAG && make juicefs.ceph && mv juicefs.ceph juicefs && upx juicefs

FROM ${BASE_IMAGE}

WORKDIR /app

ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_AUTO_UPGRADE=${JFS_AUTO_UPGRADE:-enabled}
ENV JFS_MOUNT_PATH=/usr/local/juicefs/mount/jfsmount

RUN yum install -y librados2 curl fuse && \
    rm -rf /var/cache/yum/* && \
    curl -sSL https://juicefs.com/static/juicefs -o ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI} && \
    mkdir -p /root/.juicefs

COPY --from=builder /workspace/juicefs-csi-driver/bin/juicefs-csi-driver /bin/
COPY --from=builder /workspace/juicefs/juicefs /usr/local/bin/

RUN ln -s /usr/local/bin/juicefs /bin/mount.juicefs
COPY THIRD-PARTY /

RUN /usr/bin/juicefs version && /usr/local/bin/juicefs --version

ENTRYPOINT ["/bin/juicefs-csi-driver"]
