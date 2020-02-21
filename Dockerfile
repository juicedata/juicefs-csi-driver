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

FROM golang:1.11.4-stretch as builder
WORKDIR /go/src/github.com/juicedata/juicefs-csi-driver
ENV GO111MODULE on
COPY . .
RUN make

FROM amazonlinux:2
WORKDIR /app

ENV JUICEFS_CLI=/bin/juicefs

RUN yum install util-linux -y
RUN curl -sSL https://juicefs.com/static/juicefs -o ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI}

COPY --from=builder /go/src/github.com/juicedata/juicefs-csi-driver/bin/juicefs-csi-driver /bin/juicefs-csi-driver
COPY THIRD-PARTY /

RUN juicefs version

ENV JFS_AUTO_UPGRADE=true
ENTRYPOINT ["/bin/juicefs-csi-driver"]
