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
#
IMAGE=juicedata/juicefs-csi-driver
BUILDER=juicedata/juicefs-csi-driver-builder
REGISTRY=docker.io
VERSION=0.1.0

.PHONY: juicefs-csi-driver
juicefs-csi-driver:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X github.com/juicedata/juicefs-csi-driver/pkg/driver.vendorVersion=${VERSION}" -o bin/juicefs-csi-driver ./cmd/

.PHONY: verify
verify:
	./hack/verify-all

.PHONY: test
test:
	go test -v -race ./pkg/...

.PHONY: builder
builder:
	docker build -t $(BUILDER):latest . -f builder.Dockerfile
	docker tag $(BUILDER):latest $(REGISTRY)/$(BUILDER):latest
	docker push $(REGISTRY)/$(BUILDER):latest

.PHONY: image
image:
	docker build -t $(IMAGE):latest .

.PHONY: push
push:
	docker tag $(IMAGE):latest $(REGISTRY)/$(IMAGE):latest
	docker push $(REGISTRY)/$(IMAGE):latest

.PHONY: image-release
image-release:
	docker build -t $(IMAGE):$(VERSION)

.PHONY: push-release
push-release:
	docker push $(IMAGE):$(VERSION)

.PHONY: apply
apply:
	kustomize build kubernetes | kubectl apply -f -
