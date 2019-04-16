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
WORKDIR /go/src/github.com/juicedata/juicedata-jfs-csi-driver
ENV GO111MODULE on
ADD . .
RUN make

FROM amazonlinux:2
# TODO(yujunz): install juicedata-juicefs-utils
# RUN yum install util-linux amazon-efs-utils -y
COPY --from=builder /go/src/github.com/juicedata/juicedata-jfs-csi-driver/bin/juicedata-jfs-csi-driver /bin/juicedata-jfs-csi-driver
COPY THIRD-PARTY /

ENTRYPOINT ["/bin/juicedata-jfs-csi-driver"]
