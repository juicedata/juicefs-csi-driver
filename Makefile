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

IMAGE?=juicedata/juicefs-csi-driver
REGISTRY?=docker.io
TARGETARCH?=amd64
FUSE_IMAGE=juicedata/juicefs-fuse
JUICEFS_IMAGE?=juicedata/mount
VERSION=$(shell git describe --tags --match 'v*' --always --dirty)
GIT_BRANCH?=$(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT?=$(shell git rev-parse HEAD)
DEV_TAG=dev-$(shell git describe --always --dirty)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
PKG=github.com/juicedata/juicefs-csi-driver
LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"
GO111MODULE=on
JUICEFS_CSI_LATEST_VERSION=$(shell git describe --tags --match 'v*' | grep -oE 'v[0-9]+\.[0-9][0-9]*(\.[0-9]+(-[0-9a-z]+)?)?')
IMAGE_VERSION_ANNOTATED=$(IMAGE):$(VERSION)-juicefs$(shell docker run --entrypoint=/usr/bin/juicefs $(IMAGE):$(VERSION) version | cut -d' ' -f3)

JUICEFS_CE_LATEST_VERSION=$(shell curl -fsSL https://api.github.com/repos/juicedata/juicefs/releases/latest | grep tag_name | grep -oE 'v[0-9]+\.[0-9][0-9]*(\.[0-9]+(-[0-9a-z]+)?)?')
JUICEFS_EE_LATEST_VERSION=$(shell curl -sSL https://juicefs.com/static/juicefs -o juicefs-ee && chmod +x juicefs-ee && ./juicefs-ee version | cut -d' ' -f3)
JFS_CHAN=${JFSCHAN}
JUICEFS_REPO_URL?=https://github.com/juicedata/juicefs
JUICEFS_REPO_REF?=$(JUICEFS_CE_LATEST_VERSION)
MOUNT_TAG=${MOUNTTAG}
CE_VERSION=${CEVERSION}
CE_JUICEFS_VERSION=${CEJUICEFS_VERSION}
EE_VERSION=${EEVERSION}

GOPROXY=https://goproxy.io
GOPATH=$(shell go env GOPATH)
GOOS=$(shell go env GOOS)
GOBIN=$(shell pwd)/bin

.PHONY: juicefs-csi-driver
juicefs-csi-driver:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -ldflags ${LDFLAGS} -o bin/juicefs-csi-driver ./cmd/

.PHONY: verify
verify:
	./hack/verify-all

.PHONY: test
test:
	go test -gcflags=all=-l -v -race -cover ./pkg/... -coverprofile=cov1.out

.PHONY: test-sanity
test-sanity:
	go test -v -cover ./tests/sanity/... -coverprofile=cov2.out

# build nightly image
.PHONY: image-nightly
image-nightly:
	# Build image with newest juicefs-csi-driver and juicefs
	docker build --build-arg TARGETARCH=$(TARGETARCH) \
		--build-arg JUICEFS_CE_MOUNT_IMAGE=$(JUICEFS_IMAGE):ce-nightly \
		--build-arg JUICEFS_EE_MOUNT_IMAGE=$(JUICEFS_IMAGE):ee-nightly \
		--build-arg JFSCHAN=$(JFS_CHAN) \
        -t $(IMAGE):nightly -f docker/Dockerfile .

# push nightly image
.PHONY: image-nightly-push
image-nightly-push:
	docker push $(IMAGE):nightly

# build latest image
# deprecated
.PHONY: image-latest
image-latest:
	# Build image with latest stable juicefs-csi-driver and juicefs
	docker build --build-arg JUICEFS_CSI_REPO_REF=$(JUICEFS_CSI_LATEST_VERSION) \
		--build-arg JUICEFS_REPO_REF=$(JUICEFS_CE_LATEST_VERSION) \
		--build-arg JFS_AUTO_UPGRADE=disabled \
		--build-arg TARGETARCH=$(TARGETARCH) \
		-t $(IMAGE):latest -f docker/Dockerfile .

# push latest image
# deprecated
.PHONY: push-latest
push-latest:
	docker tag $(IMAGE):latest $(REGISTRY)/$(IMAGE):latest
	docker push $(REGISTRY)/$(IMAGE):latest

# build & push csi version image
.PHONY: image-version
image-version:
	docker buildx build -t $(IMAGE):$(VERSION) \
        --build-arg JUICEFS_REPO_REF=$(CE_JUICEFS_VERSION) \
		--build-arg JUICEFS_CE_MOUNT_IMAGE=$(JUICEFS_IMAGE):$(CE_VERSION) \
		--build-arg JUICEFS_EE_MOUNT_IMAGE=$(JUICEFS_IMAGE):$(EE_VERSION) \
		--platform linux/amd64,linux/arm64 -f docker/Dockerfile . --push

.PHONY: push-version
push-version:
	docker push $(IMAGE):$(VERSION)
	docker tag $(IMAGE):$(VERSION) $(IMAGE_VERSION_ANNOTATED)
	docker push $(IMAGE_VERSION_ANNOTATED)

# build deploy yaml
yaml:
	echo "# DO NOT EDIT: generated by 'kustomize build'" > deploy/k8s.yaml
	kustomize build deploy/kubernetes/release >> deploy/k8s.yaml
	cp deploy/k8s.yaml deploy/k8s_before_v1_18.yaml
	sed -i.orig 's@storage.k8s.io/v1@storage.k8s.io/v1beta1@g' deploy/k8s_before_v1_18.yaml

	echo "# DO NOT EDIT: generated by 'kustomize build'" > deploy/webhook.yaml
	kustomize build deploy/kubernetes/webhook >> deploy/webhook.yaml
	echo "# DO NOT EDIT: generated by 'kustomize build'" > deploy/webhook-with-certmanager.yaml
	kustomize build deploy/kubernetes/webhook-with-certmanager >> deploy/webhook-with-certmanager.yaml
	./hack/update_install_script.sh

.PHONY: deploy
deploy: yaml
	kubectl apply -f $<

.PHONY: deploy-delete
uninstall: yaml
	kubectl delete -f $<

# build dev image
.PHONY: image-dev
image-dev: juicefs-csi-driver
	docker pull $(IMAGE):nightly
	docker build --build-arg TARGETARCH=$(TARGETARCH) -t $(IMAGE):$(DEV_TAG) -f docker/dev.Dockerfile bin

# push dev image
.PHONY: push-dev
push-dev:
ifeq ("$(DEV_K8S)", "microk8s")
	docker image save -o juicefs-csi-driver-$(DEV_TAG).tar $(IMAGE):$(DEV_TAG)
	sudo microk8s.ctr image import juicefs-csi-driver-$(DEV_TAG).tar
	rm -f juicefs-csi-driver-$(DEV_TAG).tar
else ifeq ("$(DEV_K8S)", "kubeadm")
	docker tag $(IMAGE):$(DEV_TAG) $(DEV_REGISTRY):$(DEV_TAG)
	docker push $(DEV_REGISTRY):$(DEV_TAG)
else
	minikube cache add $(IMAGE):$(DEV_TAG)
endif

# build image for release check
.PHONY: image-release-check
image-release-check:
	# Build image with release juicefs
	echo JUICEFS_RELEASE_CHECK_VERSION=$(JUICEFS_RELEASE_CHECK_VERSION)
	docker build --build-arg JUICEFS_CSI_REPO_REF=master \
        --build-arg JUICEFS_REPO_REF=$(CE_JUICEFS_VERSION) \
		--build-arg TARGETARCH=$(TARGETARCH) \
		--build-arg JFSCHAN=$(JFS_CHAN) \
		--build-arg JUICEFS_CE_MOUNT_IMAGE=$(JUICEFS_IMAGE):$(CE_VERSION) \
		--build-arg JUICEFS_EE_MOUNT_IMAGE=$(JUICEFS_IMAGE):$(EE_VERSION) \
		-t $(IMAGE):$(DEV_TAG) -f docker/Dockerfile .

# push image for release check
.PHONY: image-release-check-push
image-release-check-push:
	docker image save -o juicefs-csi-driver-$(DEV_TAG).tar $(IMAGE):$(DEV_TAG)
	sudo microk8s.ctr image import juicefs-csi-driver-$(DEV_TAG).tar
	rm -f juicefs-csi-driver-$(DEV_TAG).tar
	docker tag $(IMAGE):$(DEV_TAG) $(REGISTRY)/$(IMAGE):$(MOUNT_TAG)
	docker push $(REGISTRY)/$(IMAGE):$(MOUNT_TAG)

# build & push csi slim image
.PHONY: csi-slim-image-version
csi-slim-image-version:
	docker buildx build -f docker/csi.Dockerfile -t $(REGISTRY)/$(IMAGE):$(VERSION)-slim \
        --build-arg JUICEFS_REPO_REF=$(JUICEFS_CE_LATEST_VERSION) \
		--build-arg JUICEFS_CE_MOUNT_IMAGE=$(JUICEFS_IMAGE):$(CE_VERSION) \
		--build-arg JUICEFS_EE_MOUNT_IMAGE=$(JUICEFS_IMAGE):$(EE_VERSION) \
		--platform linux/amd64,linux/arm64 . --push

# build & push juicefs nightly image
.PHONY: ce-image
ce-image:
	docker build -f docker/ce.juicefs.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(CE_VERSION) \
        --build-arg JUICEFS_REPO_REF=$(CE_JUICEFS_VERSION) --build-arg TARGETARCH=$(TARGETARCH) .
	docker push $(REGISTRY)/$(JUICEFS_IMAGE):$(CE_VERSION)

.PHONY: ce-image-buildx
ce-image-buildx:
	docker buildx build -f docker/ce.juicefs.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(CE_VERSION) \
	    --build-arg=JUICEFS_REPO_REF=$(CE_JUICEFS_VERSION) \
		--platform linux/amd64,linux/arm64 . --push

.PHONY: ee-image
ee-image:
	docker build -f docker/ee.juicefs.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(EE_VERSION) \
        --build-arg=JFS_AUTO_UPGRADE=disabled --build-arg JFSCHAN=$(JFS_CHAN) --build-arg TARGETARCH=$(TARGETARCH) .
	docker push $(REGISTRY)/$(JUICEFS_IMAGE):$(EE_VERSION)

.PHONY: ee-image-buildx
ee-image-buildx:
	docker buildx build -f docker/ee.juicefs.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(EE_VERSION) \
	    --build-arg JFSCHAN=$(JFS_CHAN) \
		--platform linux/amd64,linux/arm64 . --push

# build & push image for fluid fuse
.PHONY: fuse-image-version
fuse-image-version:
	docker buildx build -f docker/fuse.Dockerfile -t $(REGISTRY)/$(FUSE_IMAGE):$(JUICEFS_CE_LATEST_VERSION)-$(JUICEFS_EE_LATEST_VERSION) \
        --build-arg JUICEFS_REPO_REF=$(JUICEFS_CE_LATEST_VERSION) \
		--build-arg=JFS_AUTO_UPGRADE=disabled --platform linux/amd64,linux/arm64 . --push

# build & push juicefs fuse nightly image
.PHONY: fuse-image-nightly
fuse-image-nightly:
	docker build -f docker/fuse.Dockerfile -t $(REGISTRY)/$(FUSE_IMAGE):nightly \
        --build-arg JUICEFS_REPO_REF=main \
		--build-arg JFSCHAN=$(JFS_CHAN) \
		--build-arg TARGETARCH=$(TARGETARCH) \
		--build-arg=JFS_AUTO_UPGRADE=disabled .
	docker push $(REGISTRY)/$(FUSE_IMAGE):nightly

.PHONY: deploy-dev/kustomization.yaml
deploy-dev/kustomization.yaml:
	mkdir -p $(@D)
	touch $@
	cd $(@D); kustomize edit add resource ../deploy/kubernetes/release;
ifeq ("$(DEV_K8S)", "kubeadm")
	cd $(@D); kustomize edit set image juicedata/juicefs-csi-driver=$(DEV_REGISTRY):$(DEV_TAG)
else
	cd $(@D); kustomize edit set image juicedata/juicefs-csi-driver=:$(DEV_TAG)
endif

deploy-dev/k8s.yaml: deploy-dev/kustomization.yaml deploy/kubernetes/release/*.yaml
	echo "# DO NOT EDIT: generated by 'kustomize build $(@D)'" > $@
	kustomize build $(@D) >> $@
	# Add .orig suffix only for compactiblity on macOS
ifeq ("$(DEV_K8S)", "microk8s")
	sed -i 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' $@
endif
ifeq ("$(DEV_K8S)", "kubeadm")
	sed -i.orig 's@juicedata/juicefs-csi-driver.*$$@$(DEV_REGISTRY):$(DEV_TAG)@g' $@
else
	sed -i.orig 's@juicedata/juicefs-csi-driver.*$$@juicedata/juicefs-csi-driver:$(DEV_TAG)@g' $@
endif

.PHONY: deploy-dev
deploy-dev: deploy-dev/k8s.yaml
	kapp deploy --app juicefs-csi-driver --file $<

.PHONY: delete-dev
delete-dev: deploy-dev/k8s.yaml
	kapp delete --app juicefs-csi-driver

.PHONY: install-dev
install-dev: verify test image-dev push-dev deploy-dev/k8s.yaml deploy-dev

bin/mockgen: | bin
	go install github.com/golang/mock/mockgen@v1.5.0

mockgen: bin/mockgen
	./hack/update-gomock

npm-install:
	npm install
	npm ci

check-docs:
	npm run markdown-lint
	autocorrect --fix ./docs/
	npm run check-broken-link
