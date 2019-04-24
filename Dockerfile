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

FROM juicedata/juicefs-csi-driver-builder:latest as builder
ENV JUICEFS_CLI=/bin/juicefs
RUN curl --silent --location https://juicefs.com/static/juicefs -o ${JUICEFS_CLI}
RUN chmod +x ${JUICEFS_CLI}

FROM amazonlinux:2
WORKDIR /app

ENV JUICEFS_CLI=/bin/juicefs

RUN yum install util-linux -y

COPY --from=builder ${JUICEFS_CLI} ${JUICEFS_CLI}
COPY --from=builder /go/src/github.com/juicedata/juicefs-csi-driver/bin/juicefs-csi-driver /bin/juicefs-csi-driver
COPY THIRD-PARTY /

RUN juicefs version

ENTRYPOINT ["/bin/juicefs-csi-driver"]
