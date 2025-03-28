# Copyright 2023 Juicedata Inc
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

FROM golang:1.23-alpine as builder
ARG GOPROXY
ARG HTTPS_PROXY
ARG HTTP_PROXY
WORKDIR /workspace
COPY --from=project **/*.go ./
COPY --from=project cmd ./cmd
COPY --from=project pkg ./pkg
COPY --from=project go.mod .
COPY --from=project go.sum .
COPY --from=project Makefile .
RUN apk add --no-cache make && make dashboard

FROM alpine:3.18
COPY --from=ui dist /dist
COPY --from=builder /workspace/bin/juicefs-csi-dashboard /usr/local/bin/juicefs-csi-dashboard
ENTRYPOINT ["juicefs-csi-dashboard"]
