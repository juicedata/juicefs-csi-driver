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

FROM golang:1.13.9-alpine3.11 as builder

RUN apk add git make

WORKDIR /juicefs-csi-driver
COPY . .
RUN make

FROM python:2.7-alpine

ARG JFS_AUTO_UPGRADE

WORKDIR /app

ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_AUTO_UPGRADE=${JFS_AUTO_UPGRADE:-enabled}

RUN apk add --update-cache \
    curl \
    util-linux \
    && rm -rf /var/cache/apk/*

RUN curl -sSL https://juicefs.com/static/juicefs -o ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI}
RUN ln -s /usr/local/bin/python /usr/bin/python

COPY --from=builder /juicefs-csi-driver/bin/juicefs-csi-driver /bin/juicefs-csi-driver
COPY THIRD-PARTY /

RUN juicefs version

ENTRYPOINT ["/bin/juicefs-csi-driver"]
