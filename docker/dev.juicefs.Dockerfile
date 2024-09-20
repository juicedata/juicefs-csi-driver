# Copyright 2021 Juicedata Inc
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

FROM golang:1.20-buster as binaryimage

ARG GOPROXY
ARG TARGETARCH=amd64

RUN bash -c "if [[ ${TARGETARCH} == amd64 ]]; then mkdir -p /home/travis/.m2 && \
    wget -O /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb https://github.com/apple/foundationdb/releases/download/6.3.23/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    dpkg -i /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    wget -O - https://download.gluster.org/pub/gluster/glusterfs/10/rsa.pub | apt-key add - && \
    echo deb [arch=${TARGETARCH}] https://download.gluster.org/pub/gluster/glusterfs/10/LATEST/Debian/buster/${TARGETARCH}/apt buster main > /etc/apt/sources.list.d/gluster.list && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    echo deb https://download.ceph.com/debian-15.2.17/ buster main | tee /etc/apt/sources.list.d/ceph.list && \
    apt-get update && apt-get install -y uuid-dev libglusterfs-dev glusterfs-common librados2 librados-dev; fi"

WORKDIR /workspace
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
COPY . .
RUN apt-get update && apt-get install -y musl-tools upx-ucl && \
    bash -c "if [[ ${TARGETARCH} == amd64 ]]; then make juicefs.all && mv juicefs.all juicefs && upx juicefs; else make juicefs; fi" && \
    mv juicefs /usr/local/bin/juicefs

FROM juicedata/mount:ce-nightly

COPY --from=binaryimage /usr/local/bin/juicefs /usr/local/bin/juicefs

RUN rm -rf /bin/mount.juicefs && ln -s /usr/local/bin/juicefs /bin/mount.juicefs && /usr/local/bin/juicefs --version
