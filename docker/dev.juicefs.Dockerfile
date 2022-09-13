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

FROM golang:1.17-buster as builder

ARG GOPROXY

WORKDIR /workspace
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
COPY . .
RUN apt-get update && apt-get install -y musl-tools upx-ucl librados-dev && \
    make juicefs.ceph && mv juicefs.ceph juicefs

FROM juicedata/mount:nightly

COPY --from=builder /workspace/juicefs /usr/local/bin/
RUN /usr/local/bin/juicefs --version
