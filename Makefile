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
REGISTRY=docker.io
VERSION=$(shell git describe --tags --match 'v*' --always --dirty)
GIT_BRANCH?=$(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT?=$(shell git rev-parse HEAD)
DEV_TAG=dev-$(shell git describe --always --dirty)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
PKG=github.com/juicedata/juicefs-csi-driver
LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"
GO111MODULE=on
IMAGE_VERSION_ANNOTATED=$(IMAGE):$(VERSION)-juicefs$(shell docker run --entrypoint=juicefs $(IMAGE):$(VERSION) version | cut -d' ' -f3)

.PHONY: juicefs-csi-driver
juicefs-csi-driver:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -ldflags ${LDFLAGS} -o bin/juicefs-csi-driver ./cmd/

.PHONY: verify
verify:
	./hack/verify-all

.PHONY: test
test:
	go test -v -race ./pkg/...

.PHONY: test-sanity
test-sanity:
	go test -v ./tests/sanity/...

.PHONY: image
image:
	docker build -t $(IMAGE):latest .

.PHONY: push
push:
	docker tag $(IMAGE):latest $(REGISTRY)/$(IMAGE):latest
	docker push $(REGISTRY)/$(IMAGE):latest

.PHONY: image-branch
image-branch:
	docker build -t $(IMAGE):$(GIT_BRANCH) -f Dockerfile .

.PHONY: push-branch
push-branch:
	docker tag $(IMAGE):$(GIT_BRANCH) $(REGISTRY)/$(IMAGE):$(GIT_BRANCH)
	docker push $(REGISTRY)/$(IMAGE):$(GIT_BRANCH)

.PHONY: image-version
image-version:
	[ -z `git status --porcelain` ] || (git --no-pager diff && exit 255)
	docker build -t $(IMAGE):$(VERSION) --build-arg=JFS_AUTO_UPGRADE=disabled .

.PHONY: push-version
push-version:
	docker push $(IMAGE):$(VERSION)
	docker tag $(IMAGE):$(VERSION) $(IMAGE_VERSION_ANNOTATED)
	docker push $(IMAGE_VERSION_ANNOTATED)

deploy/k8s.yaml: deploy/kubernetes/base/*.yaml
	echo "# DO NOT EDIT: generated by 'kustomize build'" > $@
	kustomize build deploy/kubernetes/base >> $@

.PHONY: deploy
deploy: deploy/k8s.yaml
	kubectl apply -f $<

.PHONY: deploy-delete
uninstall: deploy/k8s.yaml
	kubectl delete -f $<

.PHONY: image-dev
image-dev: juicefs-csi-driver
	docker build -t $(IMAGE):$(DEV_TAG) -f dev.Dockerfile bin

.PHONY: push-dev
push-dev:
ifeq ("$(DEV_K8S)", "microk8s")
	docker image save -o juicefs-csi-driver-$(DEV_TAG).tar $(IMAGE):$(DEV_TAG)
	microk8s.ctr image import juicefs-csi-driver-$(DEV_TAG).tar
	rm -f juicefs-csi-driver-$(DEV_TAG).tar
else
	minikube cache add $(IMAGE):$(DEV_TAG)
endif

.PHONY: deploy-dev/kustomization.yaml
deploy-dev/kustomization.yaml:
	mkdir -p $(@D)
	touch $@
	cd $(@D); kustomize edit add resource ../deploy/kubernetes/base;
	cd $(@D); kustomize edit set image juicedata/juicefs-csi-driver=:$(DEV_TAG)

deploy-dev/k8s.yaml: deploy-dev/kustomization.yaml deploy/kubernetes/base/*.yaml
	echo "# DO NOT EDIT: generated by 'kustomize build $(@D)'" > $@
	kustomize build $(@D) >> $@

.PHONY: deploy-dev
deploy-dev: deploy-dev/k8s.yaml
	kapp deploy --app juicefs-csi-driver --file $<

.PHONY: delete-dev
delete-dev: deploy-dev/k8s.yaml
	kapp delete --app juicefs-csi-driver
