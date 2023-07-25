FROM golang:1.19-buster as builder

ARG GOPROXY
ARG TARGETARCH
ARG JUICEFS_REPO_BRANCH=main
ARG JUICEFS_REPO_REF=${JUICEFS_REPO_BRANCH}

RUN bash -c "if [[ ${TARGETARCH} == amd64 ]]; then mkdir -p /home/travis/.m2 && \
    wget -O /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb https://github.com/apple/foundationdb/releases/download/6.3.23/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    dpkg -i /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    wget -O - https://download.gluster.org/pub/gluster/glusterfs/7/rsa.pub | apt-key add - && \
    echo deb [arch=${TARGETARCH}] https://download.gluster.org/pub/gluster/glusterfs/7/LATEST/Debian/buster/${TARGETARCH}/apt buster main > /etc/apt/sources.list.d/gluster.list && \
    apt-get update && apt-get install -y uuid-dev libglusterfs-dev glusterfs-common; fi"

WORKDIR /workspace
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org}
RUN apt-get update && apt-get install -y musl-tools upx-ucl librados-dev libcephfs-dev librbd-dev && \
    cd /workspace && git clone --branch=$JUICEFS_REPO_BRANCH https://github.com/juicedata/juicefs && \
    cd juicefs && git checkout $JUICEFS_REPO_REF && go get github.com/ceph/go-ceph@v0.4.0 && go mod tidy && \
    bash -c "if [[ ${TARGETARCH} == amd64 ]]; then make juicefs.all && mv juicefs.all juicefs; else make juicefs.ceph && mv juicefs.ceph juicefs; fi"

FROM python:3.8-slim-buster

ARG JFS_AUTO_UPGRADE
ARG TARGETARCH
ARG JFSCHAN

WORKDIR /app

ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_AUTO_UPGRADE=${JFS_AUTO_UPGRADE:-enabled}
ENV JFS_MOUNT_PATH=/usr/local/juicefs/mount/jfsmount
ENV JFSCHAN=${JFSCHAN}

RUN apt update && apt install -y software-properties-common wget gnupg gnupg2 && apt update && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    apt-add-repository 'deb https://download.ceph.com/debian-pacific/ buster main' && apt update && \
    bash -c "if [[ ${TARGETARCH} == amd64 ]]; then mkdir -p /home/travis/.m2 && \
    wget -O /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb https://github.com/apple/foundationdb/releases/download/6.3.23/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    dpkg -i /home/travis/.m2/foundationdb-clients_6.3.23-1_${TARGETARCH}.deb && \
    wget -O - https://download.gluster.org/pub/gluster/glusterfs/7/rsa.pub | apt-key add - && \
    echo deb [arch=${TARGETARCH}] https://download.gluster.org/pub/gluster/glusterfs/7/LATEST/Debian/buster/${TARGETARCH}/apt buster main > /etc/apt/sources.list.d/gluster.list && \
    apt-get update && apt-get install -y uuid-dev libglusterfs-dev glusterfs-common; fi"

RUN apt-get update && apt-get install -y librados2 librados-dev libcephfs-dev librbd-dev curl fuse procps iputils-ping strace iproute2 net-tools tcpdump lsof && \
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
