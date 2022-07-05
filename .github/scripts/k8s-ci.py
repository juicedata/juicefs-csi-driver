import base64
import logging
import os
import pathlib
import random
import string
import subprocess

import time
from kubernetes import client, watch, config
from kubernetes.dynamic.exceptions import ConflictError

KUBE_SYSTEM = "default"
META_URL = os.getenv("JUICEFS_META_URL") or ""
ACCESS_KEY = os.getenv("JUICEFS_ACCESS_KEY") or ""
SECRET_KEY = os.getenv("JUICEFS_SECRET_KEY") or ""
STORAGE = os.getenv("JUICEFS_STORAGE") or ""
BUCKET = os.getenv("JUICEFS_BUCKET") or ""
TOKEN = os.getenv("JUICEFS_TOKEN") or ""
IS_CE = os.getenv("IS_CE") == "True"
RESOURCE_PREFIX = "ce-" if IS_CE else "ee-"
FORMAT = '%(asctime)s %(message)s'
logging.basicConfig(format=FORMAT)
LOG = logging.getLogger('main')
LOG.setLevel(logging.INFO)

SECRET_NAME = os.getenv("JUICEFS_NAME") or "ce-juicefs-secret"
STORAGECLASS_NAME = "ce-juicefs-sc" if IS_CE else "ee-juicefs-sc"

SECRETs = []
STORAGECLASSs = []
DEPLOYMENTs = []
PODS = []
PVCs = []
PVs = []


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

    def delete(self):
        client.CoreV1Api().delete_namespaced_secret(name=self.secret_name, namespace=self.namespace)
        SECRETs.remove(self)


class StorageClass:
    def __init__(self, *, name, secret_name, parameters=None, options=None):
        self.name = name
        self.secret_name = secret_name
        self.secret_namespace = KUBE_SYSTEM
        self.parameters = parameters
        self.mount_options = options

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
            }
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
    def __init__(self, *, name, access_mode, storage_name, pv):
        self.name = RESOURCE_PREFIX + name
        self.namespace = "default"
        self.access_mode = access_mode
        self.storage_class = storage_name
        self.pv = pv

    def create(self):
        spec = client.V1PersistentVolumeClaimSpec(
            access_modes=[self.access_mode],
            resources=client.V1ResourceRequirements(
                requests={"storage": "1Gi"}
            )
        )
        if self.pv != "":
            spec.selector = client.V1LabelSelector(match_labels={"pv": self.pv})
        spec.storage_class_name = self.storage_class
        pvc = client.V1PersistentVolumeClaim(
            api_version="v1",
            kind="PersistentVolumeClaim",
            metadata=client.V1ObjectMeta(name=self.name),
            spec=spec
        )
        client.CoreV1Api().create_namespaced_persistent_volume_claim(namespace=self.namespace, body=pvc)
        PVCs.append(self)

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
        try:
            p = client.CoreV1Api().read_namespaced_persistent_volume_claim(name=self.name, namespace=self.namespace)
            pv_name = p.spec.volume_name
            pv = client.CoreV1Api().read_persistent_volume(name=pv_name)
            return pv.spec.csi.volume_handle
        except Exception as e:
            die(e)


class PV:
    def __init__(self, *, name, access_mode, volume_handle, secret_name, parameters=None, options=None):
        self.name = RESOURCE_PREFIX + name
        self.access_mode = access_mode
        self.volume_handle = volume_handle
        self.secret_name = secret_name
        self.secret_namespace = KUBE_SYSTEM
        self.parameters = parameters
        self.mount_options = options

    def create(self):
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
                volume_attributes=self.parameters,
            )
        )
        pv = client.V1PersistentVolume(
            api_version="v1",
            kind="PersistentVolume",
            metadata=client.V1ObjectMeta(name=self.name, labels={"pv": self.name}),
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


class Deployment:
    def __init__(self, *, name, pvc, replicas, out_put=""):
        self.name = RESOURCE_PREFIX + name
        self.namespace = "default"
        self.image = "centos"
        self.pvc = pvc
        self.replicas = replicas
        self.out_put = out_put

    def create(self):
        cmd = "while true; do echo $(date -u) >> /data/out.txt; sleep 1; done"
        if self.out_put != "":
            cmd = "while true; do echo $(date -u) >> /data/{}; sleep 1; done".format(self.out_put)
        container = client.V1Container(
            name="app",
            image="centos",
            command=["/bin/sh"],
            args=["-c", cmd],
            volume_mounts=[client.V1VolumeMount(
                name="juicefs-pv",
                mount_path="/data",
                mount_propagation="HostToContainer",
            )]
        )
        template = client.V1PodTemplateSpec(
            metadata=client.V1ObjectMeta(labels={"deployment": self.name}),
            spec=client.V1PodSpec(
                containers=[container],
                volumes=[client.V1Volume(
                    name="juicefs-pv",
                    persistent_volume_claim=client.V1PersistentVolumeClaimVolumeSource(claim_name=self.pvc)
                )]),
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


class Pod:
    def __init__(self, name, deployment_name, replicas, namespace="default", pvc="", out_put=""):
        self.name = name
        self.namespace = namespace
        self.deployment = deployment_name
        self.pods = []
        self.replicas = replicas
        self.image = "centos"
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
            image="centos",
            command=["sh", "-c", cmd],
            volume_mounts=[client.V1VolumeMount(
                name="juicefs-pv",
                mount_path="/data",
                mount_propagation="HostToContainer",
            )]
        )
        pod = client.V1Pod(
            metadata=client.V1ObjectMeta(name=self.name, namespace=self.namespace),
            spec=client.V1PodSpec(
                containers=[container],
                volumes=[client.V1Volume(
                    name="juicefs-pv",
                    persistent_volume_claim=client.V1PersistentVolumeClaimVolumeSource(claim_name=self.pvc)
                )]),
        )
        client.CoreV1Api().create_namespaced_pod(namespace=self.namespace, body=pod)
        PODS.append(self)

    def get_id(self):
        try:
            po = client.CoreV1Api().read_namespaced_pod(self.name, self.namespace)
            return po.metadata.uid
        except client.exceptions.ApiException as e:
            raise e


def mount_on_host(mount_path):
    LOG.info(f"Mount {mount_path}")
    try:
        if IS_CE:
            subprocess.check_call(
                ["sudo", "/usr/local/bin/juicefs", "format", f"--storage={STORAGE}", f"--access-key={ACCESS_KEY}",
                 f"--secret-key={SECRET_KEY}", f"--bucket={BUCKET}", META_URL, SECRET_NAME])
            subprocess.check_call(["sudo", "/usr/local/bin/juicefs", "mount", "-d", META_URL, mount_path])
        else:
            subprocess.check_call(
                ["sudo", "/usr/bin/juicefs", "auth", f"--token={TOKEN}", f"--accesskey={ACCESS_KEY}",
                 f"--secretkey={SECRET_KEY}", f"--bucket={BUCKET}", SECRET_NAME])
            subprocess.check_call(["sudo", "/usr/bin/juicefs", "mount", "-d", SECRET_NAME, mount_path])
        LOG.info("Mount success.")
    except Exception as e:
        LOG.info("Error in juicefs mount: {}".format(e))
        raise e


def check_mount_point(mount_path, check_path):
    mount_on_host(mount_path)
    for i in range(0, 60):
        try:
            LOG.info("Open file {}".format(check_path))
            f = open(check_path)
            content = f.read(1)
            if content is not None and content != "":
                f.close()
                LOG.info(f"Umount {mount_path}.")
                subprocess.run(["sudo", "umount", mount_path])
                return True
            time.sleep(5)
            f.close()
        except FileNotFoundError:
            LOG.info(os.listdir(mount_path))
            LOG.info("Can't find file: {}".format(check_path))
            time.sleep(5)
            continue
        except Exception as e:
            LOG.info(e)
            log = open("/var/log/juicefs.log", "rt")
            LOG.info(log.read())
            raise e
    LOG.info(f"Umount {mount_path}.")
    subprocess.run(["sudo", "umount", mount_path])
    return False


def wait_dir_empty(check_path):
    LOG.info(f"check path {check_path} empty")
    for i in range(0, 60):
        output = subprocess.run(["sudo", "ls", check_path], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if output.stderr.decode("utf-8") != "":
            LOG.info("output stderr {}".format(output.stderr.decode("utf-8")))
            return True
        if output.stdout.decode("utf-8") == "":
            return True
        time.sleep(5)

    return False


def wait_dir_not_empty(check_path):
    LOG.info(f"check path {check_path} not empty")
    for i in range(0, 60):
        output = subprocess.run(["sudo", "ls", check_path], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if output.stderr.decode("utf-8") != "":
            LOG.info("output stderr {}".format(output.stderr.decode("utf-8")))
            continue
        if output.stdout.decode("utf-8") != "":
            return True
        time.sleep(5)
    return False


def get_mount_pod_name(volume_id):
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace=KUBE_SYSTEM,
        label_selector="volume-id={}".format(volume_id)
    )
    if len(pods.items) == 0:
        die(Exception("Can't get mount pod of volume id {}".format(volume_id)))
    return pods.items[0].metadata.name


def check_mount_pod_refs(pod_name, replicas):
    pod = client.CoreV1Api().read_namespaced_pod(name=pod_name, namespace=KUBE_SYSTEM)
    annotations = pod.metadata.annotations
    if annotations is None:
        if replicas == 0:
            return True
        else:
            return False
    num = 0
    for k, v in annotations.items():
        if k.startswith("juicefs-") and "/var/lib/kubelet/pods" in v:
            num += 1
    return num == replicas


def deploy_secret_and_sc():
    LOG.info("Deploy secret & storageClass..")
    secret = Secret(secret_name=SECRET_NAME)
    secret.create()
    LOG.info("Deploy secret {}".format(secret.secret_name))
    sc = StorageClass(name=STORAGECLASS_NAME, secret_name=secret.secret_name)
    sc.create()
    LOG.info("Deploy storageClass {}".format(sc.name))


def tear_down():
    LOG.info("Tear down all resources begin..")
    try:
        for po in PODS:
            LOG.info("Delete pod {}".format(po.name))
            po.delete()
            LOG.info("Watch for pods {} for delete.".format(po.name))
            result = po.watch_for_delete(1)
            if not result:
                raise Exception("Pods {} are not delete within 5 min.".format(po.name))
        for deploy in DEPLOYMENTs:
            LOG.info("Delete deployment {}".format(deploy.name))
            deploy = deploy.refresh()
            deploy.delete()
            pod = Pod(name="", deployment_name=deploy.name, replicas=deploy.replicas)
            LOG.info("Watch for pods of deployment {} for delete.".format(deploy.name))
            result = pod.watch_for_delete(deploy.replicas)
            if not result:
                raise Exception("Pods of deployment {} are not delete within 5 min.".format(deploy.name))
        for pvc in PVCs:
            LOG.info("Delete pvc {}".format(pvc.name))
            pvc.delete()
        for sc in STORAGECLASSs:
            LOG.info("Delete storageclass {}".format(sc.name))
            sc.delete()
        for pv in PVs:
            LOG.info("Delete pv {}".format(pv.name))
            pv.delete()
        for secret in SECRETs:
            LOG.info("Delete secret {}".format(secret.secret_name))
            secret.delete()
        LOG.info("Delete all volumes in file system.")
        clean_juicefs_volume("/mnt/jfs")
    except Exception as e:
        LOG.info("Error in tear down: {}".format(e))
    LOG.info("Tear down success.")


def clean_juicefs_volume(mount_path):
    mount_on_host(mount_path)
    subprocess.run(["sudo", "rm", "-rf", mount_path + "/*"])
    subprocess.run(["sudo", "umount", mount_path])


def die(e):
    csi_node_name = os.getenv("JUICEFS_CSI_NODE_POD")
    po = Pod(name=csi_node_name, deployment_name="", replicas=1, namespace=KUBE_SYSTEM)
    LOG.info("Get csi node log:")
    LOG.info(po.get_log("juicefs-plugin"))
    LOG.info("Get csi controller log:")
    controller_po = Pod(name="juicefs-csi-controller-0", deployment_name="", replicas=1, namespace=KUBE_SYSTEM)
    LOG.info(controller_po.get_log("juicefs-plugin"))
    LOG.info("Get event: ")
    subprocess.run(["sudo", "microk8s.kubectl", "get", "event", "--all-namespaces"], check=True)
    LOG.info("Get pvc: ")
    subprocess.run(["sudo", "microk8s.kubectl", "get", "pvc", "--all-namespaces"], check=True)
    LOG.info("Get pv: ")
    subprocess.run(["sudo", "microk8s.kubectl", "get", "pv"], check=True)
    LOG.info("Get sc: ")
    subprocess.run(["sudo", "microk8s.kubectl", "get", "sc"], check=True)
    LOG.info("Get job: ")
    subprocess.run(["sudo", "microk8s.kubectl", "get", "job", "--all-namespaces"], check=True)
    raise Exception(e)


def gen_random_string(slen=10):
    return ''.join(random.sample(string.ascii_letters + string.digits, slen))


###### test case in ci ######
def test_deployment_using_storage_rw():
    LOG.info("[test case] Deployment using storageClass with rwm begin..")
    # deploy pvc
    pvc = PVC(name="pvc-dynamic-rw", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    deployment = Deployment(name="app-dynamic-rw", pvc=pvc.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    mount_path = "/mnt/jfs"
    check_path = mount_path + "/" + volume_id + "/out.txt"
    result = check_mount_point(mount_path, check_path)
    if not result:
        die("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))
    LOG.info("Test pass.")

    # delete test resources
    LOG.info("Remove deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(deployment.replicas)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    return


def test_deployment_using_storage_ro():
    LOG.info("[test case] Deployment using storageClass with rom begin..")
    # deploy pvc
    pvc = PVC(name="pvc-dynamic-ro", access_mode="ReadOnlyMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    deployment = Deployment(name="app-dynamic-ro", pvc=pvc.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    # delete test resources
    LOG.info("Remove deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(deployment.replicas)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    LOG.info("Test pass.")
    return


def test_deployment_use_pv_rw():
    LOG.info("[test case] Deployment using pv with rwm begin..")
    # deploy pv
    pv = PV(name="pv-rw", access_mode="ReadWriteMany", volume_handle="pv-rw", secret_name=SECRET_NAME)
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-static-rw", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-static-rw", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pv.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    mount_path = "/mnt/jfs"
    check_path = mount_path + "/" + out_put
    result = check_mount_point(mount_path, check_path)
    if not result:
        die("Mount point of /mnt/jfs/{} are not ready within 5 min.".format(out_put))

    # delete test resources
    LOG.info("Remove deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(deployment.replicas)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    LOG.info("Test pass.")
    return


def test_deployment_use_pv_ro():
    LOG.info("[test case] Deployment using pv with rwo begin..")
    # deploy pv
    pv = PV(name="pv-ro", access_mode="ReadOnlyMany", volume_handle="pv-ro", secret_name=SECRET_NAME)
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-static-ro", access_mode="ReadOnlyMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-static-ro", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    # delete test resources
    LOG.info("Remove deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(deployment.replicas)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    LOG.info("Test pass.")
    return


def test_delete_one():
    LOG.info("[test case] Deployment with 3 replicas begin..")
    # deploy pvc
    pvc = PVC(name="pvc-replicas", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    deployment = Deployment(name="app-replicas", pvc=pvc.name, replicas=3)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))

    # check mount pod refs
    mount_pod_name = get_mount_pod_name(volume_id)
    LOG.info("Check mount pod {} refs.".format(mount_pod_name))
    result = check_mount_pod_refs(mount_pod_name, 3)
    if not result:
        die("Mount pod {} does not have {} juicefs- refs.".format(mount_pod_name, 3))

    # update replicas = 1
    LOG.info("Set deployment {} replicas to 1".format(deployment.name))
    deployment.update_replicas(1)
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(2)
    if not result:
        die("Pods of deployment {} are not delete within 5 min.".format(deployment.name))
    # check mount pod refs
    result = check_mount_pod_refs(mount_pod_name, 1)
    LOG.info("Check mount pod {} refs.".format(mount_pod_name))
    if not result:
        raise Exception("Mount pod {} does not have {} juicefs- refs.".format(mount_pod_name, 1))

    # delete test resources
    LOG.info("Remove deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(1)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    LOG.info("Test pass.")
    return


def test_delete_all():
    LOG.info("[test case] Deployment and delete it begin..")
    # deploy pvc
    pvc = PVC(name="pvc-delete-deploy", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    deployment = Deployment(name="app-delete-deploy", pvc=pvc.name, replicas=3)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))

    # check mount pod refs
    mount_pod_name = get_mount_pod_name(volume_id)
    LOG.info("Check mount pod {} refs.".format(mount_pod_name))
    result = check_mount_pod_refs(mount_pod_name, 3)
    if not result:
        die("Mount pod {} does not have {} juicefs- refs.".format(mount_pod_name, 3))

    # delete deploy
    LOG.info("Delete deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(3)
    if not result:
        die("Pods of deployment {} are not delete within 5 min.".format(deployment.name))

    # check mount pod is delete or not
    LOG.info("Check mount pod {} is deleted or not.".format(mount_pod_name))
    pod = Pod(name=mount_pod_name, deployment_name="", replicas=1)
    result = pod.is_deleted()
    if not result:
        die("Mount pod {} does not been deleted within 5 min.".format(mount_pod_name))

    # delete test resources
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    LOG.info("Test pass.")
    return


def test_delete_pvc():
    LOG.info("[test case] Deployment and delete pvc begin..")
    # deploy pvc
    pvc = PVC(name="pvc-delete", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    deployment = Deployment(name="app-delete-pvc", pvc=pvc.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    mount_path = "/mnt/jfs"
    check_path = mount_path + "/" + volume_id + "/out.txt"
    result = check_mount_point(mount_path, check_path)
    if not result:
        die("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    LOG.info("Development delete..")
    deployment.delete()
    LOG.info("Watch deployment deleteed..")
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(1)
    if not result:
        die("Pods of deployment {} are not delete within 5 min.".format(deployment.name))

    LOG.info("PVC delete..")
    pvc.delete()
    for i in range(0, 60):
        if pvc.check_is_deleted():
            LOG.info("PVC is deleted.")
            break
        time.sleep(5)

    LOG.info("Check dir is deleted or not..")
    mount_on_host("/mnt/jfs")
    file_exist = True
    for i in range(0, 60):
        f = pathlib.Path("/mnt/jfs/" + volume_id)
        if f.exists() is False:
            file_exist = False
            break
        time.sleep(5)
    if file_exist:
        die("SubPath of volume_id {} still exists.".format(volume_id))
    LOG.info("Umount /mnt/jfs.")
    subprocess.run(["sudo", "umount", "/mnt/jfs"])

    LOG.info("Test pass.")


def test_dynamic_delete_pod():
    LOG.info("[test case] Deployment with dynamic storage and delete pod begin..")
    # deploy pvc
    pvc = PVC(name="pvc-dynamic-available", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    pod = Pod(name="app-dynamic-available", deployment_name="", replicas=1, namespace="default", pvc=pvc.name)
    pod.create()
    LOG.info("Watch for pod {} for success.".format(pod.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 5 min.".format(pod.name))
    app_pod_id = pod.get_id()

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    mount_path = "/mnt/jfs"
    check_path = mount_path + "/" + volume_id + "/out.txt"
    result = check_mount_point(mount_path, check_path)
    if not result:
        die("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    LOG.info("Mount pod delete..")
    mount_pod = Pod(name=get_mount_pod_name(volume_id), deployment_name="", replicas=1, namespace=KUBE_SYSTEM)
    mount_pod.delete()
    LOG.info("Wait for a sec..")
    time.sleep(5)

    # watch mount pod recovery
    LOG.info("Watch mount pod recovery..")
    is_ready = False
    for i in range(0, 60):
        try:
            is_ready = mount_pod.is_ready()
            if is_ready:
                break
            time.sleep(5)
        except Exception as e:
            LOG.info(e)
            raise e
    if not is_ready:
        die("Mount pod {} didn't recovery within 5 min.".format(mount_pod.name))

    LOG.info("Check mount point is ok..")
    source_path = "/var/snap/microk8s/common/var/lib/kubelet/pods/{}/volumes/kubernetes.io~csi/{}/mount".format(
        app_pod_id, volume_id)
    try:
        subprocess.check_output(["sudo", "stat", source_path], stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError as e:
        LOG.info(e)
        raise e

    # delete test resources
    LOG.info("Remove pod {}".format(pod.name))
    pod.delete()
    LOG.info("Watch for pods for delete.".format(pod.name))
    result = pod.watch_for_delete(1)
    if not result:
        raise Exception("Pods are not delete within 5 min.".format(pod.name))
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    LOG.info("Test pass.")


def test_static_delete_pod():
    LOG.info("[test case] Pod with static storage and delete mount pod begin..")
    # deploy pv
    pv = PV(name="pv-static-available", access_mode="ReadWriteMany", volume_handle="pv-static-available",
            secret_name=SECRET_NAME)
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-static-available", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    pod = Pod(name="app-static-available", deployment_name="", replicas=1, namespace="default", pvc=pvc.name,
              out_put=out_put)
    pod.create()
    LOG.info("Watch for pod {} for success.".format(pod.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 5 min.".format(pod.name))
    app_pod_id = pod.get_id()

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    mount_path = "/mnt/jfs"
    check_path = mount_path + "/" + out_put
    result = check_mount_point(mount_path, check_path)
    if not result:
        die("mount Point of /jfs/out.txt are not ready within 5 min.")

    LOG.info("Mount pod delete..")
    mount_pod = Pod(name=get_mount_pod_name(volume_id), deployment_name="", replicas=1, namespace=KUBE_SYSTEM)
    mount_pod.delete()
    LOG.info("Wait for a sec..")
    time.sleep(5)

    # watch mount pod recovery
    LOG.info("Watch mount pod recovery..")
    is_ready = False
    for i in range(0, 60):
        try:
            is_ready = mount_pod.is_ready()
            if is_ready:
                break
            time.sleep(5)
        except Exception as e:
            LOG.info(e)
            raise e
    if not is_ready:
        die("Mount pod {} didn't recovery within 5 min.".format(mount_pod.name))

    LOG.info("Check mount point is ok..")
    source_path = "/var/snap/microk8s/common/var/lib/kubelet/pods/{}/volumes/kubernetes.io~csi/{}/mount".format(
        app_pod_id, pv.name)
    try:
        subprocess.check_output(["sudo", "stat", source_path], stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError as e:
        LOG.info(e)
        raise e

    # delete test resources
    LOG.info("Remove pod {}".format(pod.name))
    pod.delete()
    LOG.info("Watch for pods for delete.".format(pod.name))
    result = pod.watch_for_delete(1)
    if not result:
        raise Exception("Pods are not delete within 5 min.".format(pod.name))
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    LOG.info("Test pass.")


def test_static_cache_clean_upon_umount():
    LOG.info("[test case] Pod with static storage and clean cache upon umount begin..")
    cache_dir = "/mnt/static/cache1:/mnt/static/cache2"
    cache_dirs = ["/mnt/static/cache1", "/mnt/static/cache2"]
    # deploy pv
    pv = PV(name="pv-static-cache-umount", access_mode="ReadWriteMany", volume_handle="pv-static-cache-umount",
            secret_name=SECRET_NAME, parameters={"juicefs/clean-cache": "true"}, options=[f"cache-dir={cache_dir}"])
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-static-cache-umount", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    pod = Pod(name="app-static-cache-umount", deployment_name="", replicas=1, namespace="default", pvc=pvc.name,
              out_put=out_put)
    pod.create()
    LOG.info("Watch for pod {} for success.".format(pod.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods of deployment {} are not ready within 5 min.".format(pod.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    mount_path = "/mnt/jfs"
    check_path = mount_path + "/" + out_put
    result = check_mount_point(mount_path, check_path)
    if not result:
        die("mount Point of /jfs/out.txt are not ready within 5 min.")

    # get volume uuid
    uuid = SECRET_NAME
    if IS_CE:
        mount_pod_name = get_mount_pod_name(volume_id)
        mount_pod = client.CoreV1Api().read_namespaced_pod(name=mount_pod_name, namespace=KUBE_SYSTEM)
        annotations = mount_pod.metadata.annotations
        if annotations is None or annotations.get("juicefs-uuid") is None:
            die("Can't get uuid of volume")
        uuid = annotations["juicefs-uuid"]
    LOG.info("Get volume uuid {}".format(uuid))

    # check cache dir not empty
    time.sleep(5)
    LOG.info("Check cache dir..")
    for cache in cache_dirs:
        not_empty = wait_dir_not_empty(f"{cache}/{uuid}/raw")
        if not not_empty:
            die("Cache empty")
    LOG.info("App pod delete..")
    pod.delete()
    LOG.info("Wait for a sec..")

    result = pod.watch_for_delete(1)
    if not result:
        raise Exception("Pods {} are not delete within 5 min.".format(pod.name))
    # check cache dir is deleted
    LOG.info("Watch cache dir clear..")
    for cache in cache_dirs:
        empty = wait_dir_empty(f"{cache}/{uuid}/raw")
        if not empty:
            die("Cache not clear")

    LOG.info("Test pass.")


def test_dynamic_cache_clean_upon_umount():
    LOG.info("[test case] Pod with dynamic storage and clean cache upon umount begin..")
    cache_dir = "/mnt/dynamic/cache1:/mnt/dynamic/cache2"
    cache_dirs = ["/mnt/dynamic/cache1", "/mnt/dynamic/cache2"]
    sc_name = RESOURCE_PREFIX + "-sc-cache"
    # deploy sc
    sc = StorageClass(name=sc_name, secret_name=SECRET_NAME,
                      parameters={"juicefs/clean-cache": "true"}, options=[f"cache-dir={cache_dir}"])
    LOG.info("Deploy storageClass {}".format(sc.name))
    sc.create()

    # deploy pvc
    pvc = PVC(name="pvc-dynamic-cache-umount", access_mode="ReadWriteMany", storage_name=sc.name, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    pod = Pod(name="app-dynamic-cache-umount", deployment_name="", replicas=1, namespace="default", pvc=pvc.name,
              out_put=out_put)
    pod.create()
    LOG.info("Watch for pod {} for success.".format(pod.name))
    result = pod.watch_for_success()
    if not result:
        die("Pods {} are not ready within 5 min.".format(pod.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    mount_path = "/mnt/jfs"
    check_path = mount_path + "/" + volume_id + "/" + out_put
    result = check_mount_point(mount_path, check_path)
    if not result:
        die("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    # get volume uuid
    uuid = SECRET_NAME
    if IS_CE:
        mount_pod_name = get_mount_pod_name(volume_id)
        mount_pod = client.CoreV1Api().read_namespaced_pod(name=mount_pod_name, namespace=KUBE_SYSTEM)
        annotations = mount_pod.metadata.annotations
        if annotations is None or annotations.get("juicefs-uuid") is None:
            die("Can't get uuid of volume")
        uuid = annotations["juicefs-uuid"]
    LOG.info("Get volume uuid {}".format(uuid))

    # check cache dir not empty
    time.sleep(5)
    LOG.info("Check cache dir..")
    for cache in cache_dirs:
        exist = wait_dir_not_empty(f"{cache}/{uuid}/raw")
        if not exist:
            subprocess.run(["sudo", "ls", f"{cache}/{uuid}/raw"])
            die("Cache empty")
    LOG.info("App pod delete..")
    pod.delete()
    LOG.info("Wait for a sec..")

    result = pod.watch_for_delete(1)
    if not result:
        raise Exception("Pods {} are not delete within 5 min.".format(pod.name))
    # check cache dir is deleted
    LOG.info("Watch cache dir clear..")
    for cache in cache_dirs:
        exist = wait_dir_empty(f"{cache}/{uuid}/raw")
        if not exist:
            die("Cache not clear")

    LOG.info("Test pass.")


def check_do_test():
    if IS_CE:
        return True
    if TOKEN == "":
        return False
    return True


if __name__ == "__main__":
    if check_do_test():
        config.load_kube_config()
        # clear juicefs volume first.
        LOG.info("clean juicefs volume first.")
        clean_juicefs_volume("/mnt/jfs")
        try:
            deploy_secret_and_sc()
            test_deployment_using_storage_rw()
            test_deployment_using_storage_ro()
            test_deployment_use_pv_rw()
            test_deployment_use_pv_ro()
            test_delete_one()
            test_delete_all()
            test_delete_pvc()
            test_dynamic_delete_pod()
            test_static_delete_pod()
            test_static_cache_clean_upon_umount()
            test_dynamic_cache_clean_upon_umount()
        finally:
            tear_down()
    else:
        LOG.info("skip test.")
