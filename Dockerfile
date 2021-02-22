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
WORKDIR /juicefs-csi-driver
COPY . .
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
RUN make

WORKDIR /workspace
RUN apt-get update && apt-get install -y musl-tools && \
    git clone --depth=1 https://github.com/juicedata/juicefs && \
    cd juicefs && STATIC=1 make

FROM python:2.7-alpine

ARG JFS_AUTO_UPGRADE

WORKDIR /app

ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_AUTO_UPGRADE=${JFS_AUTO_UPGRADE:-enabled}

RUN apk add --update-cache curl util-linux && \
    rm -rf /var/cache/apk/* && \
    curl -sSL https://juicefs.com/static/juicefs -o ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI} && \
    ln -s /usr/local/bin/python /usr/bin/python

COPY --from=builder /juicefs-csi-driver/bin/juicefs-csi-driver /workspace/juicefs/juicefs /bin/
RUN ln -s /bin/juicefs /bin/mount.juicefs
COPY THIRD-PARTY /

RUN juicefs version

ENTRYPOINT ["/bin/juicefs-csi-driver"]
