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

FROM golang:1.14 as builder

ARG GOPROXY
ARG JUICEFS_REPO_BRANCH=main
ARG JUICEFS_REPO_REF=${JUICEFS_REPO_BRANCH}
ARG JUICEFS_CSI_REPO_REF=master

WORKDIR /workspace
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
RUN apt-get update && apt-get install -y musl-tools upx-ucl && \
    git clone https://github.com/juicedata/juicefs-csi-driver && \
    cd juicefs-csi-driver && git checkout $JUICEFS_CSI_REPO_REF && make && \
    cd /workspace && git clone --branch=$JUICEFS_REPO_BRANCH https://github.com/juicedata/juicefs && \
    cd juicefs && git checkout $JUICEFS_REPO_REF && STATIC=1 make && upx juicefs

FROM python:2.7-alpine

ARG JFS_AUTO_UPGRADE

WORKDIR /app

ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_AUTO_UPGRADE=${JFS_AUTO_UPGRADE:-enabled}

RUN apk add --update-cache curl util-linux && \
    rm -rf /var/cache/apk/* && \
    curl -sSL https://juicefs.com/static/juicefs -o ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI} && \
    ln -s /usr/local/bin/python /usr/bin/python

COPY --from=builder /workspace/juicefs-csi-driver/bin/juicefs-csi-driver /bin/
COPY --from=builder /workspace/juicefs/juicefs /usr/local/bin/

RUN ln -s /usr/local/bin/juicefs /bin/mount.juicefs
COPY THIRD-PARTY /

RUN /usr/bin/juicefs version && /usr/local/bin/juicefs --version && \
    mkdir -p /usr/local/juicefs/mount && cp /root/.juicefs/jfsmount /usr/local/juicefs/mount/jfsmount

ENTRYPOINT ["/bin/juicefs-csi-driver"]
