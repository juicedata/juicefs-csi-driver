# Copyright 2021 Juicedata Inc
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

IMAGE?=juicedata/juicefs-csi-driver
REGISTRY?=docker.io
TARGETARCH?=amd64
JUICEFS_IMAGE?=juicedata/mount
VERSION=$(shell git describe --tags --match 'v*' --always --dirty)
DASHBOARD_IMAGE?=juicedata/csi-dashboard
DASHBOARD_TAG?=$(VERSION)
DEV_TAG=dev-$(shell git describe --always --dirty)
IMAGE_VERSION_ANNOTATED=$(IMAGE):$(VERSION)-juicefs$(shell docker run --entrypoint=/usr/bin/juicefs $(IMAGE):$(VERSION) version | cut -d' ' -f3)
JUICEFS_CE_LATEST_VERSION=$(shell curl -fsSL https://api.github.com/repos/juicedata/juicefs/releases/latest | grep tag_name | grep -oE 'v[0-9]+\.[0-9][0-9]*(\.[0-9]+(-[0-9a-z]+)?)?')
JUICEFS_EE_LATEST_VERSION=$(shell curl -sSL https://juicefs.com/static/juicefs -o juicefs-ee && chmod +x juicefs-ee && ./juicefs-ee version | cut -d' ' -f3)
JFS_CHAN=${JFSCHAN}
JFS_PKG_URL=${JFSPKGURL}
PKG_TYPE=${PKGTYPE}
ifeq ($(strip $(JFS_PKG_URL)),)
  JFS_PKG_URL := https://static.juicefs.com/release/bin_pkgs/latest_stable_full.tar.gz
endif
JUICEFS_REPO_URL?=https://github.com/juicedata/juicefs
JUICEFS_REPO_REF?=$(JUICEFS_CE_LATEST_VERSION)
MOUNT_TAG=${MOUNTTAG}
CE_VERSION=${CEVERSION}
CE_JUICEFS_VERSION=${CEJUICEFS_VERSION}
EE_VERSION=${EEVERSION}
CSI_VERSION=${CSIVERSION}

################# CSI image #######################

# build nightly image
.PHONY: image-nightly
image-nightly:
	# Build image with newest juicefs-csi-driver and juicefs
	docker build --build-arg TARGETARCH=$(TARGETARCH) \
		--build-arg JUICEFS_CE_MOUNT_IMAGE=$(JUICEFS_IMAGE):ce-nightly \
		--build-arg JUICEFS_EE_MOUNT_IMAGE=$(JUICEFS_IMAGE):ee-nightly \
		--build-arg JFSCHAN=$(JFS_CHAN) \
		--build-context project=.. \
        -t $(IMAGE):nightly -f csi.Dockerfile .

# push nightly image
.PHONY: image-nightly-push
image-nightly-push:
	docker push $(IMAGE):nightly

# build & push csi version image
.PHONY: image-version
image-version:
	docker buildx build -t $(IMAGE):$(CSI_VERSION) \
        --build-arg JUICEFS_REPO_REF=$(CE_JUICEFS_VERSION) \
		--build-arg JUICEFS_CE_MOUNT_IMAGE=$(JUICEFS_IMAGE):$(CE_VERSION) \
		--build-arg JUICEFS_EE_MOUNT_IMAGE=$(JUICEFS_IMAGE):$(EE_VERSION) \
		--build-context project=.. \
		--platform linux/amd64,linux/arm64 -f csi.Dockerfile . --push

.PHONY: push-version
push-version:
	docker push $(IMAGE):$(CSI_VERSION)
	docker tag $(IMAGE):$(CSI_VERSION) $(IMAGE_VERSION_ANNOTATED)
	docker push $(IMAGE_VERSION_ANNOTATED)

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
		--build-context project=.. \
		-t $(IMAGE):$(DEV_TAG) -f csi.Dockerfile .
	docker build --build-context project=.. --build-context ui=../dashboard-ui-v2/ -f dashboard.Dockerfile \
		-t $(REGISTRY)/$(DASHBOARD_IMAGE):$(DEV_TAG) .

# push image for release check
.PHONY: image-release-check-push
image-release-check-push:
	docker image save -o juicefs-csi-driver-$(DEV_TAG).tar $(IMAGE):$(DEV_TAG)
	sudo microk8s.ctr image import juicefs-csi-driver-$(DEV_TAG).tar
	rm -f juicefs-csi-driver-$(DEV_TAG).tar
	docker image save -o juicefs-csi-dashboard-$(DEV_TAG).tar $(REGISTRY)/$(DASHBOARD_IMAGE):$(DEV_TAG)
	sudo microk8s.ctr image import juicefs-csi-dashboard-$(DEV_TAG).tar
	rm -f juicefs-csi-dashboard-$(DEV_TAG).tar
	docker tag $(IMAGE):$(DEV_TAG) $(REGISTRY)/$(IMAGE):$(MOUNT_TAG)
	docker push $(REGISTRY)/$(IMAGE):$(MOUNT_TAG)
	docker tag $(REGISTRY)/$(DASHBOARD_IMAGE):$(DEV_TAG) $(REGISTRY)/$(DASHBOARD_IMAGE):$(MOUNT_TAG)
	docker push $(REGISTRY)/$(DASHBOARD_IMAGE):$(MOUNT_TAG)

################# JuiceFS ce image #######################

# build & push juicefs nightly image
.PHONY: ce-image
ce-image:
	docker build -f ce.juicefs.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(CE_VERSION) \
        --build-arg JUICEFS_REPO_REF=$(CE_JUICEFS_VERSION) --build-arg TARGETARCH=$(TARGETARCH) .
	docker push $(REGISTRY)/$(JUICEFS_IMAGE):$(CE_VERSION)

.PHONY: ce-image-buildx
ce-image-buildx:
	docker buildx build -f ce.juicefs.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(CE_VERSION) \
	    --build-arg=JUICEFS_REPO_REF=$(CE_JUICEFS_VERSION) \
		--platform linux/amd64,linux/arm64 . --push

.PHONY: ce-image-1.1
ce-image-1.1:
	docker build -f ce.juicefs-1.1.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(CE_VERSION) \
        --build-arg JUICEFS_REPO_REF=$(CE_JUICEFS_VERSION) --build-arg TARGETARCH=$(TARGETARCH) .
	docker push $(REGISTRY)/$(JUICEFS_IMAGE):$(CE_VERSION)

.PHONY: ce-image-1.1-buildx
ce-image-1.1-buildx:
	docker buildx build -f ce.juicefs-1.1.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(CE_VERSION) \
	    --build-arg=JUICEFS_REPO_REF=$(CE_JUICEFS_VERSION) \
		--platform linux/amd64,linux/arm64 . --push

################# JuiceFS ee image #######################

.PHONY: ee-image
ee-image:
	docker build -f ee.juicefs.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(EE_VERSION) \
        --build-arg=JFS_AUTO_UPGRADE=disabled --build-arg JFSCHAN=$(JFS_CHAN) --build-arg TARGETARCH=$(TARGETARCH) \
        --build-arg=PKG_TYPE=$(PKG_TYPE) --build-arg JFS_PKG_URL=$(JFS_PKG_URL) .
	docker push $(REGISTRY)/$(JUICEFS_IMAGE):$(EE_VERSION)

.PHONY: ee-image-buildx
ee-image-buildx:
	docker buildx build -f ee.juicefs.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(EE_VERSION) \
	    --build-arg JFSCHAN=$(JFS_CHAN) \
	    --build-arg JFS_PKG_URL=$(JFS_PKG_URL) \
        --build-arg PKG_TYPE=full \
		--platform linux/amd64,linux/arm64 . --push

.PHONY: ee-image-4.0-buildx
ee-image-4.0-buildx:
	docker buildx build -f ee.juicefs-4.0.Dockerfile -t $(REGISTRY)/$(JUICEFS_IMAGE):$(EE_VERSION) \
	    --build-arg JFSCHAN=$(JFS_CHAN) \
		--platform linux/amd64,linux/arm64 . --push

################# JuiceFS image for dashboard #######################
.PHONY: dashboard-build
dashboard-build:
	docker build --build-context project=../ --build-context ui=../dashboard-ui-v2/ -f dashboard.Dockerfile \
		-t $(REGISTRY)/$(DASHBOARD_IMAGE):$(DASHBOARD_TAG) .
	docker push $(REGISTRY)/$(DASHBOARD_IMAGE):$(DASHBOARD_TAG)

.PHONY: dashboard-buildx
dashboard-buildx:
	docker buildx build --build-context project=../ --build-context ui=../dashboard-ui-v2/ -f dashboard.Dockerfile \
	 	-t $(REGISTRY)/$(DASHBOARD_IMAGE):$(DASHBOARD_TAG) --platform linux/amd64,linux/arm64 . --push
