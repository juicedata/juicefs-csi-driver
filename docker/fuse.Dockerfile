FROM golang:1.18-buster as builder

ARG GOPROXY
ARG JUICEFS_REPO_BRANCH=main
ARG JUICEFS_REPO_REF=${JUICEFS_REPO_BRANCH}

WORKDIR /workspace
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
RUN apt-get update && apt-get install -y musl-tools upx-ucl librados-dev && \
    cd /workspace && git clone --branch=$JUICEFS_REPO_BRANCH https://github.com/juicedata/juicefs && \
    cd juicefs && git checkout $JUICEFS_REPO_REF && make juicefs.ceph && mv juicefs.ceph juicefs

FROM python:3.8-slim-buster

ARG JFS_AUTO_UPGRADE
ARG TARGETARCH
ARG JFSCHAN

WORKDIR /app

ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_AUTO_UPGRADE=${JFS_AUTO_UPGRADE:-enabled}
ENV JFS_MOUNT_PATH=/usr/local/juicefs/mount/jfsmount
ENV JFSCHAN=${JFSCHAN}

RUN apt-get update && apt-get install -y librados2 curl fuse && \
    rm -rf /var/cache/apt/* && \
    curl -sSL https://juicefs.com/static/juicefs -o ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI} && \
    mkdir -p /root/.juicefs && \
    ln -s /usr/local/bin/python /usr/bin/python && \
    mkdir /root/.acl && cp /etc/passwd /root/.acl/passwd && cp /etc/group /root/.acl/group && \
    ln -sf /root/.acl/passwd /etc/passwd && ln -sf /root/.acl/group  /etc/group

COPY --from=builder /workspace/juicefs/juicefs /usr/local/bin/

RUN ln -s /usr/local/bin/juicefs /bin/mount.juicefs
COPY THIRD-PARTY /

RUN /usr/bin/juicefs version && /usr/local/bin/juicefs --version

ENV K8S_VERSION v1.14.8
RUN curl -o /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/${TARGETARCH}/kubectl && chmod +x /usr/local/bin/kubectl
