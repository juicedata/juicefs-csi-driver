FROM python:3.8-slim-buster

ARG JFSCHAN

WORKDIR /app

ARG TARGETARCH
ENV JUICEFS_CLI=/usr/bin/juicefs
ENV JFS_MOUNT_PATH=/usr/local/juicefs/mount/jfsmount
ENV JFSCHAN=${JFSCHAN}

RUN apt update && apt install -y software-properties-common wget gnupg gnupg2 && bash -c "if [[ ${TARGETARCH} == amd64 ]]; then wget -O - https://download.gluster.org/pub/gluster/glusterfs/10/rsa.pub | apt-key add - && \
    echo deb [arch=${TARGETARCH}] https://download.gluster.org/pub/gluster/glusterfs/10/LATEST/Debian/buster/${TARGETARCH}/apt buster main > /etc/apt/sources.list.d/gluster.list && \
    apt-get update && apt-get install -y uuid-dev libglusterfs-dev glusterfs-common; fi"

RUN apt-get update && apt-get install -y librados2 curl fuse procps iputils-ping strace iproute2 net-tools tcpdump lsof librados-dev libcephfs-dev librbd-dev openssh-server openssh-client && \
    rm -rf /var/cache/apt/* && \
    curl -sSL https://juicefs.com/static/juicefs.4.9 -o ${JUICEFS_CLI} && chmod +x ${JUICEFS_CLI} && \
    mkdir -p /root/.juicefs /var/run/sshd && \
    ln -s /usr/local/bin/python /usr/bin/python && \
    mkdir /root/.acl && cp /etc/passwd /root/.acl/passwd && cp /etc/group /root/.acl/group && \
    ln -sf /root/.acl/passwd /etc/passwd && ln -sf /root/.acl/group  /etc/group

RUN /usr/bin/juicefs version

ENV K8S_VERSION v1.14.8
RUN curl -o /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/${TARGETARCH}/kubectl && chmod +x /usr/local/bin/kubectl
