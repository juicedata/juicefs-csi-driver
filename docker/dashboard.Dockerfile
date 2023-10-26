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

FROM golang:1.20-alpine as builder

ARG GOPROXY
ARG HTTPS_PROXY
ARG HTTP_PROXY
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
ENV HTTPS_PROXY=${HTTPS_PROXY:-}
ENV HTTP_PROXY=${HTTP_PROXY:-}

WORKDIR /workspace
COPY **/*.go ./
COPY cmd ./cmd
COPY pkg ./pkg
COPY go.mod .
COPY go.sum .
COPY Makefile .
RUN apk add --no-cache make && make dashboard

FROM alpine:3.18
COPY --from=builder /workspace/bin/juicefs-csi-dashboard /usr/local/bin/juicefs-csi-dashboard
COPY ./dashboard-ui/dist /dist
ENTRYPOINT ["juicefs-csi-dashboard"]
