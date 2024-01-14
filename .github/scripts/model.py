#  Copyright 2022 Juicedata Inc
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
import base64
import time

from kubernetes import client, watch
from kubernetes.dynamic.exceptions import ConflictError

from config import KUBE_SYSTEM, META_URL, ACCESS_KEY, SECRET_KEY, STORAGE, BUCKET, TOKEN, IS_CE, RESOURCE_PREFIX, \
    IN_CCI, CCI_APP_IMAGE, IN_VCI, LOG, SECRETs, STORAGECLASSs, DEPLOYMENTs, PODS, PVs, PVCs, JOBs


class Secret:
    def __init__(self, *, secret_name):
        self.secret_name = secret_name
        self.namespace = KUBE_SYSTEM
        self.meta_url = META_URL
        self.access_key = ACCESS_KEY
        self.secret_key = SECRET_KEY
        self.storage_name = STORAGE
        self.bucket = BUCKET
        self.token = TOKEN

    def create(self):
        if IS_CE:
            data = {
                "name": base64.b64encode(self.secret_name.encode('utf-8')).decode("utf-8"),
                "metaurl": base64.b64encode(self.meta_url.encode('utf-8')).decode("utf-8"),
                "access-key": base64.b64encode(self.access_key.encode('utf-8')).decode("utf-8"),
                "secret-key": base64.b64encode(self.secret_key.encode('utf-8')).decode("utf-8"),
                "storage": base64.b64encode(self.storage_name.encode('utf-8')).decode("utf-8"),
                "bucket": base64.b64encode(self.bucket.encode('utf-8')).decode("utf-8"),
            }
        else:
            data = {
                "name": base64.b64encode(self.secret_name.encode('utf-8')).decode("utf-8"),
                "token": base64.b64encode(self.token.encode('utf-8')).decode("utf-8"),
                "accesskey": base64.b64encode(self.access_key.encode('utf-8')).decode("utf-8"),
                "secretkey": base64.b64encode(self.secret_key.encode('utf-8')).decode("utf-8"),
                "storage": base64.b64encode(self.storage_name.encode('utf-8')).decode("utf-8"),
                "bucket": base64.b64encode(self.bucket.encode('utf-8')).decode("utf-8"),
            }
        sec = client.V1Secret(
            api_version="v1",
            kind="Secret",
            metadata=client.V1ObjectMeta(name=self.secret_name),
            data=data
        )
        client.CoreV1Api().create_namespaced_secret(namespace=self.namespace, body=sec)
        SECRETs.append(self)

    def watch_for_initconfig_injection(self):
        injected = False
        for _ in range(3):
            secret = client.CoreV1Api().read_namespaced_secret(name=self.secret_name, namespace=self.namespace)
            injected = "initconfig" in secret.data
            if injected:
                break
            time.sleep(1)

        if not injected:
            raise Exception(f"init config not found in {self.namespace}/{self.secret_name}")

    def delete(self):
        client.CoreV1Api().delete_namespaced_secret(name=self.secret_name, namespace=self.namespace)
        SECRETs.remove(self)


class StorageClass:
    def __init__(self, *, name, secret_name, parameters=None, options=None):
        self.name = name
        self.secret_name = secret_name
        self.secret_namespace = KUBE_SYSTEM
        self.parameters = parameters
        self.mount_options = ["buffer-size=300", "cache-size=100", "enable-xattr"]
        if options:
            self.mount_options.extend(options)

    def create(self):
        sc = client.V1StorageClass(
            api_version="storage.k8s.io/v1",
            kind="StorageClass",
            metadata=client.V1ObjectMeta(name=self.name),
            provisioner="csi.juicefs.com",
            reclaim_policy="Delete",
            volume_binding_mode="Immediate",
            mount_options=self.mount_options,
            parameters={
                "csi.storage.k8s.io/node-publish-secret-name": self.secret_name,
                "csi.storage.k8s.io/node-publish-secret-namespace": self.secret_namespace,
                "csi.storage.k8s.io/provisioner-secret-name": self.secret_name,
                "csi.storage.k8s.io/provisioner-secret-namespace": self.secret_namespace,
                "csi.storage.k8s.io/controller-expand-secret-name": self.secret_name,
                "csi.storage.k8s.io/controller-expand-secret-namespace": self.secret_namespace,
                "juicefs/mount-cpu-limit": "5",
                "juicefs/mount-memory-limit": "5Gi",
                "juicefs/mount-cpu-request": "100m",
                "juicefs/mount-memory-request": "500Mi",
            },
            allow_volume_expansion=True,
        )
        if self.parameters:
            for k, v in self.parameters.items():
                sc.parameters[k] = v
        client.StorageV1Api().create_storage_class(body=sc)
        STORAGECLASSs.append(self)

    def delete(self):
        client.StorageV1Api().delete_storage_class(name=self.name)
        STORAGECLASSs.remove(self)


class PVC:
    def __init__(self, *, name, access_mode, storage_name, pv, labels=None, annotations=None):
        if labels is None:
            labels = {}
        self.name = RESOURCE_PREFIX + name
        self.namespace = "default"
        self.access_mode = access_mode
        self.storage_class = storage_name
        self.pv = pv
        self.labels = labels
        self.annotations = annotations
        self.capacity = "1Gi"

    def create(self):
        spec = client.V1PersistentVolumeClaimSpec(
            access_modes=[self.access_mode],
            resources=client.V1ResourceRequirements(
                requests={"storage": self.capacity}
            )
        )
        if self.pv != "":
            spec.selector = client.V1LabelSelector(match_labels={"pv": self.pv})
        spec.storage_class_name = self.storage_class
        pvc = client.V1PersistentVolumeClaim(
            api_version="v1",
            kind="PersistentVolumeClaim",
            metadata=client.V1ObjectMeta(name=self.name, labels=self.labels, annotations=self.annotations),
            spec=spec
        )
        client.CoreV1Api().create_namespaced_persistent_volume_claim(namespace=self.namespace, body=pvc)
        PVCs.append(self)

    def update_capacity(self, capacity):
        pvc = client.CoreV1Api().read_namespaced_persistent_volume_claim(name=self.name, namespace=self.namespace)
        pvc.spec.resources = client.V1ResourceRequirements(
            requests={"storage": capacity}
        )
        client.CoreV1Api().replace_namespaced_persistent_volume_claim(
            name=self.name, namespace=self.namespace, body=pvc
        )

    def delete(self):
        client.CoreV1Api().delete_namespaced_persistent_volume_claim(name=self.name, namespace=self.namespace)
        PVCs.remove(self)

    def check_is_deleted(self):
        try:
            client.CoreV1Api().read_namespaced_persistent_volume_claim(name=self.name, namespace=self.namespace)
        except client.exceptions.ApiException as e:
            if e.status == 404:
                return True
            raise e
        return False

    def get_volume_id(self):
        p = client.CoreV1Api().read_namespaced_persistent_volume_claim(name=self.name, namespace=self.namespace)
        pv_name = p.spec.volume_name
        pv = client.CoreV1Api().read_persistent_volume(name=pv_name)
        return pv.spec.csi.volume_handle

    def check_is_bound(self):
        p = client.CoreV1Api().read_namespaced_persistent_volume_claim(name=self.name, namespace=self.namespace)
        pv_name = p.spec.volume_name
        if pv_name is not None and pv_name != "":
            return True
        return False


class PV:
    def __init__(self, *, name, access_mode, volume_handle, secret_name,
                 parameters=None, options=None, annotation=None):
        self.name = RESOURCE_PREFIX + name
        self.access_mode = access_mode
        self.volume_handle = volume_handle
        self.secret_name = secret_name
        self.secret_namespace = KUBE_SYSTEM
        self.parameters = parameters
        self.annotation = annotation
        self.mount_options = ["cache-size=100", "enable-xattr", "verbose"]
        if options:
            self.mount_options.extend(options)

    def create(self):
        parameters = {
            "juicefs/mount-cpu-limit": "5",
            "juicefs/mount-memory-limit": "5Gi",
            "juicefs/mount-cpu-request": "100m",
            "juicefs/mount-memory-request": "500Mi",
        }
        if self.parameters is not None:
            for k, v in self.parameters.items():
                parameters[k] = v
        spec = client.V1PersistentVolumeSpec(
            access_modes=[self.access_mode],
            capacity={"storage": "10Pi"},
            volume_mode="Filesystem",
            persistent_volume_reclaim_policy="Delete",
            mount_options=self.mount_options,
            csi=client.V1CSIPersistentVolumeSource(
                driver="csi.juicefs.com",
                fs_type="juicefs",
                volume_handle=self.volume_handle,
                node_publish_secret_ref=client.V1SecretReference(
                    name=self.secret_name,
                    namespace=self.secret_namespace
                ),
                volume_attributes=parameters,
            )
        )
        pv = client.V1PersistentVolume(
            api_version="v1",
            kind="PersistentVolume",
            metadata=client.V1ObjectMeta(name=self.name, labels={"pv": self.name}, annotations=self.annotation),
            spec=spec
        )
        client.CoreV1Api().create_persistent_volume(body=pv)
        PVs.append(self)

    def delete(self):
        client.CoreV1Api().delete_persistent_volume(name=self.name)
        PVs.remove(self)

    def get_volume_id(self):
        p = client.CoreV1Api().read_persistent_volume(name=self.name)
        return p.spec.csi.volume_handle

    def get_volume_status(self):
        p = client.CoreV1Api().read_persistent_volume(name=self.name)
        return p.status

    def get_volume(self):
        p = client.CoreV1Api().read_persistent_volume(name=self.name)
        return p

    def patch_mount_options(self):
        p = client.CoreV1Api().patch_persistent_volume(name=self.name)
        return p.spec.csi.volume_handle


class Deployment:
    def __init__(self, *, name, pvc, replicas, out_put="", pvcs=[]):
        self.name = RESOURCE_PREFIX + name
        self.namespace = "default"
        self.image = "centos"
        if IN_CCI:
            self.image = CCI_APP_IMAGE
        self.replicas = replicas
        self.out_put = out_put
        self.pvcs = [pvc]
        if pvcs:
            self.pvcs = pvcs

    def create(self):
        output = "out.txt"
        if self.out_put:
            output = self.out_put
        volume_mounts = []
        volumes = []
        date_cmds = []
        for i, pvc in enumerate(self.pvcs):
            volume_mounts.append(client.V1VolumeMount(
                name="juicefs-pv-{}".format(i),
                mount_path="/data-{}".format(i),
                mount_propagation="HostToContainer",
            ))
            volumes.append(client.V1Volume(
                name="juicefs-pv-{}".format(i),
                persistent_volume_claim=client.V1PersistentVolumeClaimVolumeSource(claim_name=pvc)
            ))
            date_cmds.append("echo $(date -u) >> /data-{}/{};".format(i, output))
        date_cmd = " ".join(date_cmds)
        cmd = "while true; do {} sleep 1; done".format(date_cmd)
        container = client.V1Container(
            name="app",
            image=self.image,
            command=["/bin/sh"],
            args=["-c", cmd],
            volume_mounts=volume_mounts,
        )
        template = client.V1PodTemplateSpec(
            metadata=client.V1ObjectMeta(labels={"deployment": self.name}),
            spec=client.V1PodSpec(
                containers=[container],
                volumes=volumes,
            )
        )
        if IN_CCI:
            template.metadata = client.V1ObjectMeta(
                labels={
                    "deployment": self.name,
                    "virtual-kubelet.io/burst-to-cci": "enforce",
                }
            )
            container.resources = client.V1ResourceRequirements(
                limits={
                    "cpu": "1",
                    "memory": "1Gi",
                },
                requests={
                    "cpu": "1",
                    "memory": "1Gi",
                }
            )
            template.spec.containers = [container]
        if IN_VCI:
            template.metadata = client.V1ObjectMeta(
                labels={"deployment": self.name},
                annotations={"vke.volcengine.com/burst-to-vci": "enforce"}
            )
        deploySpec = client.V1DeploymentSpec(
            replicas=self.replicas,
            template=template,
            selector={"matchLabels": {"deployment": self.name}}
        )
        deploy = client.V1Deployment(
            api_version="apps/v1",
            kind="Deployment",
            metadata=client.V1ObjectMeta(name=self.name),
            spec=deploySpec,
        )
        client.AppsV1Api().create_namespaced_deployment(namespace=self.namespace, body=deploy)
        DEPLOYMENTs.append(self)

    def update_replicas(self, replicas):
        while True:
            try:
                deployment = client.AppsV1Api().read_namespaced_deployment(name=self.name, namespace=self.namespace)
                deployment.spec.replicas = replicas
                client.AppsV1Api().patch_namespaced_deployment(name=self.name, namespace=self.namespace,
                                                               body=deployment)
            except (client.ApiException, ConflictError) as e:
                if e.reason == "Conflict":
                    LOG.error(e)
                    continue
            break

    def delete(self):
        client.AppsV1Api().delete_namespaced_deployment(name=self.name, namespace=self.namespace)
        DEPLOYMENTs.remove(self)

    def refresh(self):
        deploy = client.AppsV1Api().read_namespaced_deployment(name=self.name, namespace=self.namespace)
        self.replicas = deploy.spec.replicas
        return self


class Job:
    def __init__(self, *, name, pvc, out_put=""):
        self.name = RESOURCE_PREFIX + name
        self.namespace = "default"
        self.image = "centos"
        if IN_CCI:
            self.image = CCI_APP_IMAGE
        self.pvc = pvc
        self.out_put = out_put

    def create(self):
        cmd = "echo $(date -u) >> /data/out.txt; sleep 10;"
        if self.out_put != "":
            cmd = "echo $(date -u) >> /data/{}; sleep 10;".format(self.out_put)
        container = client.V1Container(
            name="app",
            image=self.image,
            command=["/bin/sh"],
            args=["-c", cmd],
            volume_mounts=[client.V1VolumeMount(
                name="juicefs-pv",
                mount_path="/data",
            )]
        )
        template = client.V1PodTemplateSpec(
            metadata=client.V1ObjectMeta(labels={"deployment": self.name}),
            spec=client.V1PodSpec(
                restart_policy="Never",
                containers=[container],
                volumes=[client.V1Volume(
                    name="juicefs-pv",
                    persistent_volume_claim=client.V1PersistentVolumeClaimVolumeSource(claim_name=self.pvc)
                )]),
        )
        if IN_CCI:
            template.metadata = client.V1ObjectMeta(
                labels={
                    "deployment": self.name,
                    "virtual-kubelet.io/burst-to-cci": "enforce",
                }
            )
            container.resources = client.V1ResourceRequirements(
                limits={
                    "cpu": "1",
                    "memory": "1Gi",
                },
                requests={
                    "cpu": "1",
                    "memory": "1Gi",
                }
            )
            template.spec.containers = [container]
        if IN_VCI:
            template.metadata = client.V1ObjectMeta(
                labels={"deployment": self.name, },
                annotations={"vke.volcengine.com/burst-to-vci": "enforce", }
            )
        job_spec = client.V1JobSpec(
            template=template,
        )
        job = client.V1Job(
            api_version="batch/v1",
            kind="Job",
            metadata=client.V1ObjectMeta(name=self.name),
            spec=job_spec,
        )
        client.BatchV1Api().create_namespaced_job(namespace=self.namespace, body=job)
        JOBs.append(self)

    def watch_for_complete(self):
        v1 = client.BatchV1Api()
        for i in range(0, 60):
            try:
                job = v1.read_namespaced_job_status(self.name, self.namespace)
                if job.status.succeeded == 1:
                    LOG.info("Job {} completed.".format(self.name))
                    return True
                if job.status.failed == 1:
                    raise Exception("Job {} failed.".format(self.name))
                time.sleep(1)
            except Exception as e:
                raise e
        return False

    def delete(self):
        client.BatchV1Api().delete_namespaced_job(name=self.name, namespace=self.namespace)
        JOBs.remove(self)


class Pod:
    def __init__(self, name, deployment_name, replicas, namespace="default", pvc="", out_put=""):
        self.name = name
        self.namespace = namespace
        self.deployment = deployment_name
        self.pods = []
        self.replicas = replicas
        self.image = "centos"
        if IN_CCI:
            self.image = CCI_APP_IMAGE
        self.pvc = pvc
        self.replicas = replicas
        self.out_put = out_put

    def watch_for_success(self):
        v1 = client.CoreV1Api()
        w = watch.Watch()
        for event in w.stream(v1.list_pod_for_all_namespaces, timeout_seconds=10 * 60):
            resource = event['object']
            if resource.metadata.namespace != "default":
                continue
            if self.name == "" and resource.metadata.labels is not None and \
                    resource.metadata.labels.get("deployment") != self.deployment:
                continue
            if self.name != "" and resource.metadata.name != self.name:
                continue
            LOG.info("Event: %s %s" % (event['type'], event['object'].metadata.name))
            if self.__is_pod_ready(resource):
                if self.name == "":
                    self.pods.append(resource)
                    if len(self.pods) == self.replicas:
                        self.pods = []
                        return True
                else:
                    return True
        return False

    @staticmethod
    def __is_pod_ready(resource):
        if resource.status.phase.lower() != "running":
            LOG.info("Pod {} status phase: {}".format(resource.metadata.name, resource.status.phase))
            return False
        conditions = resource.status.conditions
        for c in conditions:
            if c.status != "True":
                return False
        LOG.info("Pod {} status is ready.".format(resource.metadata.name))
        return True

    def watch_for_delete(self, num):
        v1 = client.CoreV1Api()
        w = watch.Watch()
        for event in w.stream(v1.list_pod_for_all_namespaces, timeout_seconds=5 * 60):
            resource = event['object']
            message_type = event['type']
            if resource.metadata.namespace != "default":
                continue
            if self.name == "" and resource.metadata.labels.get("deployment") != self.deployment:
                continue
            if self.name != "" and resource.metadata.name != self.name:
                continue
            LOG.info("Event: %s %s" % (event['type'], event['object'].metadata.name))
            if message_type == "DELETED":
                if self.name == "":
                    self.pods.append(resource)
                    if len(self.pods) == num:
                        self.pods = []
                        return True
                else:
                    return True
        return False

    def get_name(self):
        v1 = client.CoreV1Api()
        pods = v1.list_namespaced_pod(namespace=self.namespace, label_selector="deployment=" + self.deployment)
        if len(pods.items) != 0:
            return pods.items[0].metadata.name

    def is_deleted(self):
        try:
            po = client.CoreV1Api().read_namespaced_pod(self.name, self.namespace)
        except client.exceptions.ApiException as e:
            if e.status == 404:
                return True
            raise e
        return po.metadata.deletion_timestamp != ""

    def is_ready(self):
        try:
            po = client.CoreV1Api().read_namespaced_pod(self.name, self.namespace)
            return self.__is_pod_ready(po)
        except client.exceptions.ApiException as e:
            if e.status == 404:
                return False
            raise e

    def get_log(self, container_name):
        return client.CoreV1Api().read_namespaced_pod_log(self.name, self.namespace, container=container_name)

    def delete(self):
        client.CoreV1Api().delete_namespaced_pod(name=self.name, namespace=self.namespace)
        if self in PODS:
            PODS.remove(self)

    def create(self):
        cmd = "while true; do echo $(date -u) >> /data/out.txt; sleep 1; done"
        if self.out_put != "":
            cmd = "while true; do echo $(date -u) >> /data/{}; sleep 1; done".format(self.out_put)
        container = client.V1Container(
            name="app",
            image=self.image,
            command=["sh", "-c", cmd],
            volume_mounts=[client.V1VolumeMount(
                name="juicefs-pv",
                mount_path="/data",
                mount_propagation="HostToContainer",
            )]
        )
        pod = client.V1Pod(
            metadata=client.V1ObjectMeta(
                name=self.name,
                namespace=self.namespace,
            ),
            spec=client.V1PodSpec(
                containers=[container],
                volumes=[client.V1Volume(
                    name="juicefs-pv",
                    persistent_volume_claim=client.V1PersistentVolumeClaimVolumeSource(claim_name=self.pvc)
                )]),
        )
        if IN_CCI:
            pod.metadata = client.V1ObjectMeta(
                name=self.name,
                namespace=self.namespace,
                labels={
                    "virtual-kubelet.io/burst-to-cci": "enforce",
                }
            )
            container.resources = client.V1ResourceRequirements(
                limits={
                    "cpu": "1",
                    "memory": "1Gi",
                },
                requests={
                    "cpu": "1",
                    "memory": "1Gi",
                }
            )
            pod.spec.containers = [container]
        if IN_VCI:
            pod.metadata = client.V1ObjectMeta(
                name=self.name,
                namespace=self.namespace,
                annotations={
                    "vke.volcengine.com/burst-to-vci": "enforce",
                }
            )
        client.CoreV1Api().create_namespaced_pod(namespace=self.namespace, body=pod)
        PODS.append(self)

    def get_id(self):
        try:
            po = client.CoreV1Api().read_namespaced_pod(self.name, self.namespace)
            return po.metadata.uid
        except client.exceptions.ApiException as e:
            raise e

    def get_metadata(self):
        try:
            po = client.CoreV1Api().read_namespaced_pod(self.name, self.namespace)
            return po.metadata
        except client.exceptions.ApiException as e:
            raise e

    def get_spec(self):
        try:
            po = client.CoreV1Api().read_namespaced_pod(self.name, self.namespace)
            return po.spec
        except client.exceptions.ApiException as e:
            raise e
