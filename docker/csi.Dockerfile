# Copyright 2022 Juicedata Inc
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
FROM golang:1.17-buster as builder

ARG GOPROXY
ARG JUICEFS_REPO_BRANCH=main
ARG JUICEFS_REPO_REF=${JUICEFS_REPO_BRANCH}
ARG JUICEFS_CSI_REPO_REF=master
ARG TARGETARCH

ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}

WORKDIR /workspace
COPY . .
RUN make

ENV STATIC=1
RUN apt-get update && apt-get install -y musl-tools upx-ucl librados-dev && \
    cd /workspace && git clone --branch=$JUICEFS_REPO_BRANCH https://github.com/juicedata/juicefs && \
    cd juicefs && git checkout $JUICEFS_REPO_REF && make && upx juicefs
RUN upx /workspace/bin/juicefs-csi-driver

ADD https://github.com/krallin/tini/releases/download/v0.19.0/tini-${TARGETARCH} /tini
RUN chmod +x /tini

FROM alpine:3.15.5

COPY --from=builder /workspace/bin/juicefs-csi-driver /bin/juicefs-csi-driver
COPY --from=builder /workspace/juicefs/juicefs /usr/local/bin/juicefs
COPY --from=builder /tini /tini

RUN ln -s /usr/local/bin/juicefs /bin/mount.juicefs && /usr/local/bin/juicefs --version

ENTRYPOINT ["/tini", "--", "/bin/juicefs-csi-driver"]