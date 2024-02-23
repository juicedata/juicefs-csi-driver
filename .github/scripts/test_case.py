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
import os
import pathlib
import subprocess
import time

from kubernetes import client

from config import KUBE_SYSTEM, IS_CE, RESOURCE_PREFIX, \
    SECRET_NAME, STORAGECLASS_NAME, GLOBAL_MOUNTPOINT, \
    LOG, PVs, META_URL, MOUNT_MODE, CCI_MOUNT_IMAGE, IN_CCI
from model import PVC, PV, Pod, StorageClass, Deployment, Job, Secret
from util import check_mount_point, wait_dir_empty, wait_dir_not_empty, \
    get_only_mount_pod_name, get_mount_pods, check_pod_ready, check_mount_pod_refs, gen_random_string, get_vol_uuid, \
    get_voldel_job, check_quota, is_quota_supported


def test_deployment_using_storage_rw():
    LOG.info("[test case] Deployment using storageClass with rwm begin..")
    # deploy pvc
    pvc = PVC(name="pvc-dynamic-rw", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    deployment = Deployment(name="app-dynamic-rw", pvc=pvc.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))
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


def test_quota_using_storage_rw():
    if not is_quota_supported():
        LOG.info("juicefs donot support quota, skip.")
        return

    LOG.info("[test case] Quota using storageClass with rwm begin..")
    # deploy pvc
    pvc = PVC(name="pvc-quota-rw", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    deployment = Deployment(name="app-quota-rw", pvc=pvc.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    check_quota(pods.items[0].metadata.name, "1.0G")
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


# this case is not valid.
def test_deployment_using_storage_ro():
    LOG.info("[test case] Deployment using storageClass with rom begin..")
    # deploy pvc
    pvc = PVC(name="pvc-dynamic-ro", access_mode="ReadOnlyMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    deployment = Deployment(name="app-dynamic-ro", pvc=pvc.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

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

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-static-rw", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pv.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    result = check_mount_point(out_put)
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("Mount point of /mnt/jfs/{} are not ready within 5 min.".format(out_put))

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

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-static-ro", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

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
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))

    # check mount pod refs
    unique_id = volume_id
    test_mode = os.getenv("TEST_MODE")
    if test_mode == "pod-mount-share":
        unique_id = STORAGECLASS_NAME
    mount_pod_name = get_only_mount_pod_name(unique_id)
    LOG.info("Check mount pod {} refs.".format(mount_pod_name))
    result = check_mount_pod_refs(mount_pod_name, 3)
    if not result:
        raise Exception("Mount pod {} does not have {} juicefs- refs.".format(mount_pod_name, 3))

    # update replicas = 1
    LOG.info("Set deployment {} replicas to 1".format(deployment.name))
    deployment.update_replicas(1)
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(2)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))
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
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))

    # check mount pod refs
    unique_id = volume_id
    test_mode = os.getenv("TEST_MODE")
    if test_mode == "pod-mount-share":
        unique_id = STORAGECLASS_NAME
    mount_pod_name = get_only_mount_pod_name(unique_id)
    LOG.info("Check mount pod {} refs.".format(mount_pod_name))
    result = check_mount_pod_refs(mount_pod_name, 3)
    if not result:
        raise Exception("Mount pod {} does not have {} juicefs- refs.".format(mount_pod_name, 3))

    # delete deploy
    LOG.info("Delete deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(3)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))

    # check mount pod is delete or not
    LOG.info("Check mount pod {} is deleted or not.".format(mount_pod_name))
    pod = Pod(name=mount_pod_name, deployment_name="", replicas=1)
    result = pod.is_deleted()
    if not result:
        raise Exception("Mount pod {} does not been deleted within 5 min.".format(mount_pod_name))

    # delete test resources
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    LOG.info("Test pass.")
    return


def test_multi_pvc():
    LOG.info("[test case] application with multi pvcs begin..")
    # deploy pv
    volume1_handle = "multi-1"
    pv1 = PV(name="pv-multi-1", access_mode="ReadWriteMany", volume_handle=volume1_handle,
             secret_name=SECRET_NAME, annotation={"pv.kubernetes.io/provisioned-by": "csi.juicefs.com"})
    LOG.info("Deploy pv {}".format(pv1.name))
    pv1.create()

    volume2_handle = "multi-2"
    pv2 = PV(name="pv-multi-2", access_mode="ReadWriteMany", volume_handle=volume2_handle,
             secret_name=SECRET_NAME, annotation={"pv.kubernetes.io/provisioned-by": "csi.juicefs.com"})
    LOG.info("Deploy pv {}".format(pv2.name))
    pv2.create()

    # deploy pvc
    pvc1 = PVC(name="pvc-multi-1", access_mode="ReadWriteMany", storage_name="", pv=pv1.name)
    LOG.info("Deploy pvc {}".format(pvc1.name))
    pvc1.create()

    pvc2 = PVC(name="pvc-multi-2", access_mode="ReadWriteMany", storage_name="", pv=pv2.name)
    LOG.info("Deploy pvc {}".format(pvc2.name))
    pvc2.create()

    # deploy pod
    # wait for pvc bound
    for i in range(0, 60):
        if pvc1.check_is_bound() and pvc2.check_is_bound():
            break
        time.sleep(1)

    output = gen_random_string(6) + ".txt"
    # deploy pod
    deployment = Deployment(name="deploy-multi", pvc="", replicas=1, out_put=output, pvcs=[pvc1.name, pvc2.name])
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=1)
    LOG.info("Watch for pods of {} for success.".format(pod.name))
    result = pod.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    result = check_mount_point(output)
    if not result:
        raise Exception("mount Point of /jfs/{} are not ready within 5 min.".format(output))

    # check app pod label and annotation
    mount_pod1_name = get_only_mount_pod_name(volume1_handle)
    mount_pod2_name = get_only_mount_pod_name(volume2_handle)
    LOG.info("Check app pod labels and annotations.")

    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    pod = pods.items[0]
    meta = pod.metadata

    annos = meta.annotations
    if annos is None:
        annos = {}

    # unique1_id = volume1_handle
    # unique2_id = volume2_handle
    # key1 = f"juicefs-mountpod-{unique1_id}"
    # key2 = f"juicefs-mountpod-{unique2_id}"
    # mount_pod1 = annos[key1]
    # mount_pod2 = annos[key2]
    # if mount_pod1 != f"{KUBE_SYSTEM}/{mount_pod1_name}":
    #     raise Exception("App pod {} does not have [{}] annotation {}. pod annotations: {}".format(
    #         meta.name, key1, mount_pod1_name, annos))
    # if mount_pod2 != f"{KUBE_SYSTEM}/{mount_pod2_name}":
    #     raise Exception("App pod {} does not have [{}] annotation {}. pod annotations: {}".format(
    #         meta.name, key2, mount_pod2_name, annos))

    # delete test resources
    LOG.info("Remove deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(1)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))
    LOG.info("Remove pvc {}".format(pvc1.name))
    pvc1.delete()
    LOG.info("Remove pvc {}".format(pvc2.name))
    pvc2.delete()
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
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    LOG.info("Development delete..")
    deployment.delete()
    LOG.info("Watch deployment deleteed..")
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(1)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))

    LOG.info("PVC delete..")
    pvc.delete()
    for i in range(0, 60):
        if pvc.check_is_deleted():
            LOG.info("PVC is deleted.")
            break
        time.sleep(5)

    LOG.info("Check dir is deleted or not..")
    file_exist = True
    for i in range(0, 60):
        f = pathlib.Path(GLOBAL_MOUNTPOINT + "/" + volume_id)
        if f.exists() is False:
            file_exist = False
            break
        time.sleep(5)
    if file_exist:
        raise Exception("SubPath of volume_id {} still exists.".format(volume_id))

    LOG.info("Test pass.")


def test_static_delete_policy():
    LOG.info("[test case] Delete Reclaim policy of static begin..")
    volume_id = "pv-static-delete"

    LOG.info("create subdir {}".format(volume_id))
    subdir = GLOBAL_MOUNTPOINT + "/" + volume_id
    if not os.path.exists(subdir):
        os.mkdir(subdir)

    # deploy pv
    pv = PV(name="pv-static-delete", access_mode="ReadWriteMany", volume_handle=volume_id,
            secret_name=SECRET_NAME, annotation={"pv.kubernetes.io/provisioned-by": "csi.juicefs.com"})
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-static-delete", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # check pvc and pv status
    LOG.info("check pv status")
    bound = False
    for i in range(0, 60):
        pv_status = pv.get_volume_status()
        if pv_status.phase == "Bound":
            bound = True
            break
        time.sleep(5)

    if not bound:
        raise Exception("PersistentVolume {} not bound".format(pv.name))

    LOG.info("PVC delete..")
    pvc.delete()
    for i in range(0, 60):
        if pvc.check_is_deleted():
            LOG.info("PVC is deleted.")
            break
        time.sleep(5)
    PVs.remove(pv)

    LOG.info("Check dir is deleted or not..")
    file_exist = True
    for i in range(0, 60):
        f = pathlib.Path(subdir)
        if f.exists() is False:
            file_exist = False
            break
        time.sleep(5)
    if not file_exist:
        raise Exception("SubPath of static pv volume_id {} is deleted.".format(volume_id))

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
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(pod.name))
    app_pod_id = pod.get_id()

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    LOG.info("Mount pod delete..")
    unique_id = volume_id
    test_mode = os.getenv("TEST_MODE")
    if test_mode == "pod-mount-share":
        unique_id = STORAGECLASS_NAME
    mount_pod = Pod(name=get_only_mount_pod_name(unique_id), deployment_name="", replicas=1, namespace=KUBE_SYSTEM)
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
        raise Exception("Mount pod {} didn't recovery within 5 min.".format(mount_pod.name))

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
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(pod.name))
    app_pod_id = pod.get_id()

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    result = check_mount_point(out_put)
    if not result:
        raise Exception("mount Point of /jfs/out.txt are not ready within 5 min.")

    LOG.info("Mount pod delete..")
    mount_pod = Pod(name=get_only_mount_pod_name(volume_id), deployment_name="", replicas=1, namespace=KUBE_SYSTEM)
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
        raise Exception("Mount pod {} didn't recovery within 5 min.".format(mount_pod.name))

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


def test_pod_resource_err():
    LOG.info("[test case] Pod resource error begin..")
    # deploy pv
    pv = PV(name="pv-resource-err", access_mode="ReadWriteMany", volume_handle="pv-resource-err",
            secret_name=SECRET_NAME,
            parameters={"juicefs/mount-cpu-request": "10", "juicefs/mount-memory-request": "50Gi",
                        "juicefs/mount-cpu-limit": "10", "juicefs/mount-memory-limit": "50Gi", })
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-resource-err", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    pod = Pod(name="app-resource-err", deployment_name="", replicas=1, namespace="default", pvc=pvc.name,
              out_put=out_put)
    pod.create()
    LOG.info("Watch for pod {} for success.".format(pod.name))
    result = pod.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(pod.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    result = check_mount_point(out_put)
    if not result:
        raise Exception("mount Point of /jfs/out.txt are not ready within 5 min.")

    LOG.info("Check resources of mount pod..")
    mount_pod = Pod(name=get_only_mount_pod_name(volume_id), deployment_name="", replicas=1, namespace=KUBE_SYSTEM)
    spec = mount_pod.get_spec()
    resource_requests = spec.containers[0].resources.requests
    if resource_requests is not None and resource_requests["cpu"] != "" and resource_requests["memory"] != "":
        raise Exception("Mount pod {} resources request is not none.".format(mount_pod.name))

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


def test_cache_client_conf():
    LOG.info("[test case] Pod with static storage and clean cache upon umount begin..")
    secret = Secret(secret_name=SECRET_NAME)
    secret.watch_for_initconfig_injection()

def test_static_cache_clean_upon_umount():
    LOG.info("[test case] Pod with static storage and clean cache upon umount begin..")
    by_process = MOUNT_MODE == "process"

    cache_dir = "/mnt/static/cache1:/mnt/static/cache2"
    cache_dirs = ["/mnt/static/cache1", "/mnt/static/cache2"]
    if by_process:
        cache_dir = "/jfs/static/cache1:/jfs/static/cache2"
        cache_dirs = ["/var/lib/juicefs/volume/static/cache1", "/var/lib/juicefs/volume/static/cache2"]

    # deploy pv
    pv = PV(name="pv-static-cache-umount", access_mode="ReadWriteMany", volume_handle="pv-static-cache-umount",
            secret_name=SECRET_NAME, parameters={"juicefs/clean-cache": "true"}, options=[f"cache-dir={cache_dir}",f"free-space-ratio=0.01"])
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
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(pod.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    result = check_mount_point(out_put)
    if not result:
        raise Exception("mount Point of /jfs/out.txt are not ready within 5 min.")

    # get volume uuid
    uuid = SECRET_NAME
    if IS_CE:
        if not by_process:
            unique_id = volume_id
            mount_pod_name = get_only_mount_pod_name(unique_id)
            mount_pod = client.CoreV1Api().read_namespaced_pod(name=mount_pod_name, namespace=KUBE_SYSTEM)
            annotations = mount_pod.metadata.annotations
            if annotations is None or annotations.get("juicefs-uuid") is None:
                raise Exception("Can't get uuid of volume")
            uuid = annotations["juicefs-uuid"]
        else:
            uuid = get_vol_uuid(META_URL)
    LOG.info("Get volume uuid {}".format(uuid))

    # check cache dir not empty
    time.sleep(10)
    LOG.info("Check cache dir..")
    for cache in cache_dirs:
        not_empty = wait_dir_not_empty(f"{cache}/{uuid}/raw")
        if not not_empty:
            raise Exception("Cache empty")
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
            raise Exception("Cache not clear")

    LOG.info("Test pass.")


def test_dynamic_cache_clean_upon_umount():
    LOG.info("[test case] Pod with dynamic storage and clean cache upon umount begin..")
    by_process = MOUNT_MODE == "process"

    cache_dir = "/mnt/dynamic/cache1:/mnt/dynamic/cache2"
    cache_dirs = ["/mnt/dynamic/cache1", "/mnt/dynamic/cache2"]
    if by_process:
        cache_dir = "/jfs/dynamic/cache1:/jfs/dynamic/cache2"
        cache_dirs = ["/var/lib/juicefs/volume/dynamic/cache1", "/var/lib/juicefs/volume/dynamic/cache2"]

    sc_name = RESOURCE_PREFIX + "-sc-cache"
    # deploy sc
    sc = StorageClass(name=sc_name, secret_name=SECRET_NAME,
                      parameters={"juicefs/clean-cache": "true"}, options=[f"cache-dir={cache_dir}",f"free-space-ratio=0.01"])
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
        raise Exception("Pods {} are not ready within 5 min.".format(pod.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/" + out_put
    result = check_mount_point(check_path)
    if not result:
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    # get volume uuid
    uuid = SECRET_NAME
    if IS_CE:
        if not by_process:
            unique_id = volume_id
            test_mode = os.getenv("TEST_MODE")
            if test_mode == "pod-mount-share":
                unique_id = sc_name
            mount_pod_name = get_only_mount_pod_name(unique_id)
            mount_pod = client.CoreV1Api().read_namespaced_pod(name=mount_pod_name, namespace=KUBE_SYSTEM)
            annotations = mount_pod.metadata.annotations
            if annotations is None or annotations.get("juicefs-uuid") is None:
                raise Exception("Can't get uuid of volume")
            uuid = annotations["juicefs-uuid"]
        else:
            uuid = get_vol_uuid(META_URL)
    LOG.info("Get volume uuid {}".format(uuid))

    # check cache dir not empty
    time.sleep(5)
    LOG.info("Check cache dir..")
    for cache in cache_dirs:
        exist = wait_dir_not_empty(f"{cache}/{uuid}/raw")
        if not exist:
            subprocess.run(["sudo", "ls", f"{cache}/{uuid}/raw"])
            raise Exception("Cache empty")
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
            raise Exception("Cache not clear")

    LOG.info("Test pass.")


def test_deployment_dynamic_patch_pv():
    LOG.info("[test case] Deployment dynamic update pv")
    # deploy pvc
    pvc = PVC(name="pvc-dynamic-update-pv", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    deployment = Deployment(name="app-dynamic-update-pv", pvc=pvc.name, replicas=2)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    # patch pv
    subdir = gen_random_string(6)
    pv_name = volume_id
    pv = client.CoreV1Api().read_persistent_volume(name=pv_name)
    pv.spec.mount_options = ["subdir={}".format(subdir), "verbose"]
    pv.spec.mount_options.append("subdir={}".format(subdir))
    LOG.info(f"Patch PV {pv_name}: add subdir={subdir} and verbose in mountOptions")
    client.CoreV1Api().patch_persistent_volume(pv_name, pv)

    # delete one app pod
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    pod = pods.items[0]
    pod_name = pod.metadata.name
    pod_namespace = pod.metadata.namespace
    LOG.info("Delete pod {}".format(pod_name))
    client.CoreV1Api().delete_namespaced_pod(pod_name, pod_namespace)
    # wait for pod deleted
    LOG.info("Wait for pod {} deleting...".format(pod_name))
    for i in range(0, 60):
        try:
            client.CoreV1Api().read_namespaced_pod(pod_name, pod_namespace)
            time.sleep(5)
            continue
        except client.exceptions.ApiException as e:
            if e.status == 404:
                LOG.info("Pod {} has been deleted.".format(pod_name))
                break
            raise Exception(e)

    # wait for app pod ready again
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    pod_ready = True
    for i in range(0, 60):
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_ready = True
            if not check_pod_ready(po):
                pod_ready = False
                time.sleep(2)
                break
        if pod_ready:
            break

    if not pod_ready:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 2 min.".format(deployment.name))

    # check mount pod
    LOG.info("Check 2 mount pods.")
    unique_id = volume_id
    test_mode = os.getenv("TEST_MODE")
    if test_mode == "pod-mount-share":
        unique_id = STORAGECLASS_NAME
    mount_pods = get_mount_pods(unique_id)
    if len(mount_pods.items) != 2:
        raise Exception("There should be 2 mount pods, [{}] are found.".format(len(mount_pods.items)))

    # check subdir
    LOG.info("Check subdir {}".format(subdir))
    result = check_mount_point(subdir + "/{}/out.txt".format(volume_id))
    if not result:
        raise Exception("mount Point of /{}/out.txt are not ready within 5 min.".format(subdir))

    # check target
    LOG.info("Check target path is ok..")
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    for pod in pods.items:
        source_path = "/var/snap/microk8s/common/var/lib/kubelet/pods/{}/volumes/kubernetes.io~csi/{}/mount".format(
            pod.metadata.uid, volume_id)
        try:
            subprocess.check_output(["sudo", "stat", source_path], stderr=subprocess.STDOUT)
        except subprocess.CalledProcessError as e:
            raise Exception(e)

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


def test_deployment_static_patch_pv():
    LOG.info("[test case] Deployment static update pv")
    # deploy pv
    pv = PV(name="pv-update-pv", access_mode="ReadWriteMany", volume_handle="pv-update-pv", secret_name=SECRET_NAME)
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-static-update-pv", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-static-update-pv", pvc=pvc.name, replicas=2, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pv.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    result = check_mount_point(out_put)
    if not result:
        raise Exception("Mount point of /mnt/jfs/{} are not ready within 5 min.".format(out_put))

    # patch pv
    subdir = gen_random_string(6)
    pv_name = pv.name
    pv = client.CoreV1Api().read_persistent_volume(name=pv_name)
    pv.spec.mount_options.append("subdir={}".format(subdir))
    LOG.info(f"Patch PV {pv_name}: add subdir={subdir} in mountOptions")
    client.CoreV1Api().patch_persistent_volume(pv_name, pv)

    # delete one app pod
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    pod = pods.items[0]
    pod_name = pod.metadata.name
    pod_namespace = pod.metadata.namespace
    LOG.info("Delete pod {}".format(pod_name))
    client.CoreV1Api().delete_namespaced_pod(pod_name, pod_namespace)
    # wait for pod deleted
    LOG.info("Wait for pod {} deleting...".format(pod_name))
    for i in range(0, 60):
        try:
            client.CoreV1Api().read_namespaced_pod(pod_name, pod_namespace)
            time.sleep(5)
            continue
        except client.exceptions.ApiException as e:
            if e.status == 404:
                LOG.info("Pod {} has been deleted.".format(pod_name))
                break
            raise Exception(e)

    # wait for app pod ready again
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    pod_ready = True
    for i in range(0, 60):
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_ready = True
            if not check_pod_ready(po):
                pod_ready = False
                time.sleep(2)
                break
        if pod_ready:
            break

    if not pod_ready:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 2 min.".format(deployment.name))

    # check mount pod
    LOG.info("Check 2 mount pods.")
    mount_pods = get_mount_pods(volume_id)
    if len(mount_pods.items) != 2:
        raise Exception("There should be 2 mount pods, [{}] are found.".format(len(mount_pods.items)))

    # check subdir
    LOG.info("Check subdir {}".format(subdir))
    result = check_mount_point(subdir + "/" + out_put)
    if not result:
        raise Exception("mount Point of /{}/out.txt are not ready within 5 min.".format(subdir))

    # check target
    LOG.info("Check target path is ok..")
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    for pod in pods.items:
        source_path = "/var/snap/microk8s/common/var/lib/kubelet/pods/{}/volumes/kubernetes.io~csi/{}/mount".format(
            pod.metadata.uid, pv_name)
        try:
            subprocess.check_output(["sudo", "stat", source_path], stderr=subprocess.STDOUT)
        except subprocess.CalledProcessError as e:
            raise Exception(e)

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


def test_dynamic_mount_image():
    LOG.info("[test case] Deployment set mount image in storageClass begin..")
    mount_image = "juicedata/mount:ee-nightly"
    if IS_CE:
        mount_image = "juicedata/mount:ce-nightly"
    if IN_CCI:
        mount_image = CCI_MOUNT_IMAGE
    # deploy sc
    sc_name = "mount-image-dynamic"
    sc = StorageClass(name=sc_name, secret_name=SECRET_NAME,
                      parameters={"juicefs/mount-image": mount_image})
    LOG.info("Deploy storageClass {}".format(sc.name))
    sc.create()

    # deploy pvc
    pvc = PVC(name="pvc-mount-image-dynamic", access_mode="ReadWriteMany", storage_name=sc.name, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    deployment = Deployment(name="app-mount-image-dynamic", pvc=pvc.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    LOG.info("Check mount pod image")
    unique_id = volume_id
    test_mode = os.getenv("TEST_MODE")
    if test_mode == "pod-mount-share":
        unique_id = sc.name
    mount_pods = get_mount_pods(unique_id)
    if len(mount_pods.items) != 1:
        raise Exception("There should be 1 mount pods, [{}] are found.".format(len(mount_pods.items)))
    mount_pod = mount_pods.items[0]
    # check mount pod image
    if mount_pod.spec.containers[0].image != mount_image:
        raise Exception("Image of mount pod is not {}".format(mount_image))

    LOG.info("Remove deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(deployment.replicas)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))

    LOG.info("Check voldel pod image")
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    job = get_voldel_job(volume_id)
    # check voldel pod image
    if job.spec.template.spec.containers[0].image != mount_image:
        raise Exception("Image of voldel pod is not {}".format(mount_image))

    # delete test resources
    LOG.info("Remove sc {}".format(pvc.name))
    sc.delete()
    LOG.info("Test pass.")
    return


def test_static_mount_image():
    LOG.info("[test case] Deployment set mount image in PV begin..")
    mount_image = "juicedata/mount:ee-nightly"
    if IS_CE:
        mount_image = "juicedata/mount:ce-nightly"
    if IN_CCI:
        mount_image = CCI_MOUNT_IMAGE
    # deploy pv
    pv_name = "mount-image-pv"
    pv = PV(name=pv_name, access_mode="ReadWriteMany", volume_handle=pv_name,
            secret_name=SECRET_NAME, parameters={"juicefs/mount-image": mount_image})
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-mount-image-static", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-mount-image-static", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pv.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    result = check_mount_point(out_put)
    if not result:
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    LOG.info("Check mount pod image")
    mount_pods = get_mount_pods(volume_id)
    if len(mount_pods.items) != 1:
        raise Exception("There should be 1 mount pods, [{}] are found.".format(len(mount_pods.items)))
    mount_pod = mount_pods.items[0]
    # check mount pod image
    if mount_pod.spec.containers[0].image != mount_image:
        raise Exception("Image of mount pod is not {}".format(mount_image))

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
    LOG.info("Remove pv {}".format(pvc.name))
    pv.delete()
    LOG.info("Test pass.")
    return


def test_share_mount():
    LOG.info("[test case] Two Deployment using storageClass shared mount begin..")
    # deploy pvc
    pvc1 = PVC(name="pvc-share-mount-1", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc1.name))
    pvc1.create()
    pvc2 = PVC(name="pvc-share-mount-2", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc2.name))
    pvc2.create()

    # deploy pod
    deployment1 = Deployment(name="app-share-mount-1", pvc=pvc1.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment1.name))
    deployment1.create()
    pod1 = Pod(name="", deployment_name=deployment1.name, replicas=deployment1.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment1.name))
    result = pod1.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment1.name))
    deployment2 = Deployment(name="app-share-mount-2", pvc=pvc2.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment2.name))
    deployment2.create()
    pod2 = Pod(name="", deployment_name=deployment2.name, replicas=deployment2.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment2.name))
    result = pod2.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment2.name))

    # check mount pod refs
    mount_pod_name = get_only_mount_pod_name(STORAGECLASS_NAME)
    LOG.info("Check mount pod {} refs.".format(mount_pod_name))
    result = check_mount_pod_refs(mount_pod_name, 2)
    if not result:
        raise Exception("Mount pod {} does not have {} juicefs- refs.".format(mount_pod_name, 2))

    # delete test resources
    LOG.info("Remove deployment {}".format(deployment1.name))
    deployment1.delete()
    pod = Pod(name="", deployment_name=deployment1.name, replicas=deployment1.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment1.name))
    result = pod.watch_for_delete(deployment1.replicas)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment1.name))
    LOG.info("Remove pvc {}".format(pvc1.name))
    pvc1.delete()

    LOG.info("Remove deployment {}".format(deployment2.name))
    deployment2.delete()
    pod = Pod(name="", deployment_name=deployment2.name, replicas=deployment2.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment2.name))
    result = pod.watch_for_delete(deployment2.replicas)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment2.name))
    LOG.info("Remove pvc {}".format(pvc2.name))
    pvc2.delete()

    LOG.info("Test pass.")
    return


def test_path_pattern_in_storage_class():
    LOG.info("[test case] Path pattern in storageClass begin..")
    label_value = gen_random_string(3)
    anno_value = gen_random_string(3)
    # deploy sc
    sc_name = "path-pattern-dynamic"
    sc = StorageClass(
        name=sc_name, secret_name=SECRET_NAME,
        parameters={"pathPattern": "${.PVC.namespace}-${.PVC.name}-${.PVC.labels.abc}-${.PVC.annotations.abc}"})
    LOG.info("Deploy storageClass {}".format(sc.name))
    sc.create()

    # deploy pvc
    pvc = PVC(name="path-pattern-dynamic", access_mode="ReadWriteMany",
              storage_name=sc.name, pv="", labels={"abc": label_value}, annotations={"abc": anno_value})
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-path-pattern-dynamic", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    check_path = "{}-{}-{}-{}/{}".format("default", pvc.name, label_value, anno_value, out_put)
    result = check_mount_point(check_path)
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of {} are not ready within 5 min.".format(check_path))
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
    LOG.info("Remove sc {}".format(pvc.name))
    sc.delete()
    return


def test_dynamic_pvc_delete_with_path_pattern():
    LOG.info("[test case] delete pvc with path pattern in storageClass begin..")
    label_value = gen_random_string(3)
    anno_value = gen_random_string(3)
    # deploy sc
    sc_name = "delete-pvc-path-pattern-dynamic"
    sc = StorageClass(
        name=sc_name, secret_name=SECRET_NAME,
        parameters={"pathPattern": "${.PVC.namespace}-${.PVC.name}-${.PVC.labels.abc}-${.PVC.annotations.abc}"})
    LOG.info("Deploy storageClass {}".format(sc.name))
    sc.create()

    # deploy pvc
    pvc = PVC(name="delete-pvc-path-pattern-dynamic", access_mode="ReadWriteMany",
              storage_name=sc.name, pv="", labels={"abc": label_value}, annotations={"abc": anno_value})
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-delete-pvc-path-pattern-dynamic", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    check_path = "{}-{}-{}-{}/{}".format("default", pvc.name, label_value, anno_value, out_put)
    result = check_mount_point(check_path)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of {} are not ready within 5 min.".format(check_path))

    LOG.info("Development delete..")
    deployment.delete()
    LOG.info("Watch deployment deleted..")
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(1)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))

    LOG.info("PVC delete..")
    pvc.delete()
    for i in range(0, 60):
        if pvc.check_is_deleted():
            LOG.info("PVC is deleted.")
            break
        LOG.info("PVC is not deleted.")
        time.sleep(5)

    LOG.info("Check dir is deleted or not..")
    file_exist = True
    for i in range(0, 60):
        f = pathlib.Path(GLOBAL_MOUNTPOINT + "/" + check_path)
        if f.exists() is False:
            file_exist = False
            break
        time.sleep(5)

    if file_exist:
        LOG.info("Mount point dir: ")
        LOG.info(os.listdir(GLOBAL_MOUNTPOINT))
        raise Exception("SubPath of volume_id {} still exists.".format(check_path))

    LOG.info("Test pass.")
    # delete test resources
    LOG.info("Remove sc {}".format(sc.name))
    sc.delete()
    return


def test_deployment_dynamic_patch_pv_with_webhook():
    LOG.info("[test case] Deployment dynamic update pv with webhook")
    # deploy pvc
    pvc = PVC(name="pvc-dynamic-update-pv-webhook", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    deployment = Deployment(name="app-dynamic-update-pv-webhook", pvc=pvc.name, replicas=2)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            if not check_pod_ready(po):
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /{}/out.txt are not ready within 5 min.".format(volume_id))

    # patch pv
    subdir = gen_random_string(6)
    pv_name = volume_id
    pv = client.CoreV1Api().read_persistent_volume(name=pv_name)
    pv.spec.mount_options = ["subdir={}".format(subdir), "verbose"]
    pv.spec.mount_options.append("subdir={}".format(subdir))
    LOG.info(f"Patch PV {pv_name}: add subdir={subdir} and verbose in mountOptions")
    client.CoreV1Api().patch_persistent_volume(pv_name, pv)

    # delete one app pod
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    pod = pods.items[0]
    pod_name = pod.metadata.name
    pod_namespace = pod.metadata.namespace
    LOG.info("Delete pod {}".format(pod_name))
    client.CoreV1Api().delete_namespaced_pod(pod_name, pod_namespace)
    # wait for pod deleted
    LOG.info("Wait for pod {} deleting...".format(pod_name))
    for i in range(0, 60):
        try:
            client.CoreV1Api().read_namespaced_pod(pod_name, pod_namespace)
            time.sleep(5)
            continue
        except client.exceptions.ApiException as e:
            if e.status == 404:
                LOG.info("Pod {} has been deleted.".format(pod_name))
                break
            raise Exception(e)

    # wait for app pod ready again
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    pod_ready = True
    for i in range(0, 60):
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_ready = True
            if not check_pod_ready(po):
                pod_ready = False
                time.sleep(2)
                break
        if pod_ready:
            break

    if not pod_ready:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            if not check_pod_ready(po):
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 2 min.".format(deployment.name))

    # check subdir
    LOG.info("Check subdir {}".format(subdir))
    result = check_mount_point(subdir + "/{}/out.txt".format(volume_id))
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /{}/out.txt are not ready within 5 min.".format(subdir))

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


def test_deployment_static_patch_pv_with_webhook():
    LOG.info("[test case] Deployment static update pv with webhook")
    # deploy pv
    pv = PV(name="pv-update-pv-webhook", access_mode="ReadWriteMany", volume_handle="pv-update-pv-webhook",
            secret_name=SECRET_NAME)
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-static-update-pv-webhook", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-static-update-pv-webhook", pvc=pvc.name, replicas=2, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            if not check_pod_ready(po):
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 5 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pv.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    result = check_mount_point(out_put)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("Mount point of /mnt/jfs/{} are not ready within 5 min.".format(out_put))

    # patch pv
    subdir = gen_random_string(6)
    pv_name = pv.name
    pv = client.CoreV1Api().read_persistent_volume(name=pv_name)
    pv.spec.mount_options.append("subdir={}".format(subdir))
    LOG.info(f"Patch PV {pv_name}: add subdir={subdir} in mountOptions")
    client.CoreV1Api().patch_persistent_volume(pv_name, pv)

    # delete one app pod
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    pod = pods.items[0]
    pod_name = pod.metadata.name
    pod_namespace = pod.metadata.namespace
    LOG.info("Delete pod {}".format(pod_name))
    client.CoreV1Api().delete_namespaced_pod(pod_name, pod_namespace)
    # wait for pod deleted
    LOG.info("Wait for pod {} deleting...".format(pod_name))
    for i in range(0, 60):
        try:
            client.CoreV1Api().read_namespaced_pod(pod_name, pod_namespace)
            time.sleep(5)
            continue
        except client.exceptions.ApiException as e:
            if e.status == 404:
                LOG.info("Pod {} has been deleted.".format(pod_name))
                break
            raise Exception(e)

    # wait for app pod ready again
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    pod_ready = True
    for i in range(0, 60):
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_ready = True
            if not check_pod_ready(po):
                pod_ready = False
                time.sleep(2)
                break
        if pod_ready:
            break

    if not pod_ready:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            if not check_pod_ready(po):
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 2 min.".format(deployment.name))

    # check subdir
    LOG.info("Check subdir {}".format(subdir))
    result = check_mount_point(subdir + "/" + out_put)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /{}/out.txt are not ready within 5 min.".format(subdir))

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


def test_dynamic_pvc_delete_not_last_with_path_pattern():
    LOG.info("[test case] delete pvc with path pattern not last in storageClass begin..")
    label_value = gen_random_string(3)
    anno_value = gen_random_string(3)
    # deploy sc
    sc_name = "delete-pvc-path-pattern-dynamic-not-last"
    sc = StorageClass(
        name=sc_name, secret_name=SECRET_NAME,
        parameters={"pathPattern": "${.PVC.namespace}-${.PVC.labels.abc}-${.PVC.annotations.abc}"})
    LOG.info("Deploy storageClass {}".format(sc.name))
    sc.create()

    # deploy pvc
    pvc = PVC(name="delete-pvc-path-pattern-not-last", access_mode="ReadWriteMany",
              storage_name=sc.name, pv="", labels={"abc": label_value}, annotations={"abc": anno_value})
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # deploy the other pvc
    other_pvc = PVC(name="other-delete-pvc-path-pattern-not-last", access_mode="ReadWriteMany",
                    storage_name=sc.name, pv="", labels={"abc": label_value}, annotations={"abc": anno_value})
    LOG.info("Deploy pvc {}".format(pvc.name))
    other_pvc.create()

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-delete-pvc-path-pattern-not-last", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    check_path = "{}-{}-{}/{}".format("default", label_value, anno_value, out_put)
    result = check_mount_point(check_path)
    if not result:
        raise Exception("mount Point of {} are not ready within 5 min.".format(check_path))

    LOG.info("Development delete..")
    deployment.delete()
    LOG.info("Watch deployment deleted..")
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(1)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))

    LOG.info("PVC delete..")
    pvc.delete()
    for i in range(0, 60):
        if pvc.check_is_deleted():
            LOG.info("PVC is deleted.")
            break
        LOG.info("PVC is not deleted.")
        time.sleep(5)

    LOG.info("Check dir is deleted or not..")
    file_exist = True
    for i in range(0, 60):
        f = pathlib.Path(GLOBAL_MOUNTPOINT + "/" + check_path)
        if f.exists() is False:
            file_exist = False
            break
        time.sleep(5)

    if not file_exist:
        LOG.info("Mount point dir: ")
        LOG.info(os.listdir(GLOBAL_MOUNTPOINT))
        raise Exception(
            "SubPath of volume_id {} not exists, it should not be deleted because not the last".format(check_path))

    LOG.info("Test pass.")
    # delete test resources
    LOG.info("Remove sc {}".format(sc.name))
    sc.delete()
    LOG.info("Remove pvc {}".format(other_pvc.name))
    other_pvc.delete()


def test_dynamic_mount_image_with_webhook():
    LOG.info("[test case] Deployment set mount image in storageClass with webhook begin..")
    mount_image = "juicedata/mount:ee-nightly"
    if IS_CE:
        mount_image = "juicedata/mount:ce-nightly"
    if IN_CCI:
        mount_image = CCI_MOUNT_IMAGE
    # deploy sc
    sc_name = "mount-image-dynamic-webhook"
    sc = StorageClass(name=sc_name, secret_name=SECRET_NAME,
                      parameters={"juicefs/mount-image": mount_image})
    LOG.info("Deploy storageClass {}".format(sc.name))
    sc.create()

    # deploy pvc
    pvc = PVC(name="pvc-mount-image-dynamic-webhook", access_mode="ReadWriteMany", storage_name=sc.name, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    deployment = Deployment(name="app-mount-image-dynamic-webhook", pvc=pvc.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            if not check_pod_ready(po):
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    # check sidecar image
    LOG.info("Check sidecar image")
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    if len(pods.items) != 1:
        raise Exception("Pods of deployment {} are not ready.".format(deployment.name))

    pod = pods.items[0]
    found_image = ""
    for container in pod.spec.containers:
        if container.name == "jfs-mount":
            found_image = container.image

    if found_image != mount_image:
        raise Exception("Image of sidecar is not {}".format(mount_image))

    LOG.info("Remove deployment {}".format(deployment.name))
    deployment.delete()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of deployment {} for delete.".format(deployment.name))
    result = pod.watch_for_delete(deployment.replicas)
    if not result:
        raise Exception("Pods of deployment {} are not delete within 5 min.".format(deployment.name))

    LOG.info("Check voldel pod image")
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    job = get_voldel_job(volume_id)
    # check voldel pod image
    if job.spec.template.spec.containers[0].image != mount_image:
        raise Exception("Image of voldel pod is not {}".format(mount_image))

    # delete test resources
    LOG.info("Remove sc {}".format(pvc.name))
    sc.delete()
    LOG.info("Test pass.")
    return


def test_static_mount_image_with_webhook():
    LOG.info("[test case] Deployment set mount image in PV begin..")
    mount_image = "juicedata/mount:ee-nightly"
    if IS_CE:
        mount_image = "juicedata/mount:ce-nightly"
    if IN_CCI:
        mount_image = CCI_MOUNT_IMAGE
    # deploy pv
    pv_name = "mount-image-pv-webhook"
    pv = PV(name=pv_name, access_mode="ReadWriteMany", volume_handle=pv_name,
            secret_name=SECRET_NAME, parameters={"juicefs/mount-image": mount_image})
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-mount-image-static-webhook", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-mount-image-static-webhook", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            if not check_pod_ready(po):
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pv.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    result = check_mount_point(out_put)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    # check sidecar image
    LOG.info("Check sidecar image")
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    if len(pods.items) != 1:
        raise Exception("Pods of deployment {} are not ready.".format(deployment.name))

    pod = pods.items[0]
    found_image = ""
    for container in pod.spec.containers:
        if container.name == "jfs-mount":
            found_image = container.image

    if found_image != mount_image:
        raise Exception("Image of sidecar is not {}".format(mount_image))

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
    LOG.info("Remove pv {}".format(pvc.name))
    pv.delete()
    LOG.info("Test pass.")
    return


# only for webhook
def test_job_complete_using_storage():
    LOG.info("[test case] Job using storageClass with rwm begin..")
    # deploy pvc
    pvc = PVC(name="pvc-job-dynamic", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    job = Job(name="job-dynamic", pvc=pvc.name)
    LOG.info("Deploy Job {}".format(job.name))
    job.create()
    pod = Pod(name="", deployment_name=job.name, replicas=1)
    LOG.info("Watch for pods of {} for success.".format(job.name))
    result = pod.watch_for_success()
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(job.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            if not check_pod_ready(po):
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of job {} are not ready within 10 min.".format(job.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(job.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    # check job complete
    LOG.info("Check job complete..")
    result = job.watch_for_complete()
    if not result:
        raise Exception("Job {} is not complete within 1 min.".format(job.name))
    LOG.info("Test pass.")

    # delete test resources
    LOG.info("Remove job {}".format(job.name))
    job.delete()
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    return


def test_static_job_complete():
    LOG.info("[test case] Job static with rwm begin..")
    # deploy pv
    pv_name = "pv-for-job"
    pv = PV(name=pv_name, access_mode="ReadWriteMany", volume_handle=pv_name, secret_name=SECRET_NAME)
    LOG.info("Deploy pv {}".format(pv.name))
    pv.create()

    # deploy pvc
    pvc = PVC(name="pvc-job-static", access_mode="ReadWriteMany", storage_name="", pv=pv.name)
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    job = Job(name="job-static", pvc=pvc.name, out_put=out_put)
    LOG.info("Deploy Job {}".format(job.name))
    job.create()
    pod = Pod(name="", deployment_name=job.name, replicas=1)
    LOG.info("Watch for pods of {} for success.".format(job.name))
    result = pod.watch_for_success()
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(job.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            if not check_pod_ready(po):
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of job {} are not ready within 10 min.".format(job.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = out_put
    result = check_mount_point(check_path)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(job.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    # check job complete
    LOG.info("Check job complete..")
    result = job.watch_for_complete()
    if not result:
        raise Exception("Job {} is not complete within 1 min.".format(job.name))
    LOG.info("Test pass.")

    # delete test resources
    LOG.info("Remove job {}".format(job.name))
    job.delete()
    LOG.info("Remove pvc {}".format(pvc.name))
    pvc.delete()
    return


def test_webhook_two_volume():
    LOG.info("[test case] Deployment using two PVC with rwm begin..")
    # deploy pv
    pv_name1 = "pv-one"
    pv1 = PV(name=pv_name1, access_mode="ReadWriteMany", volume_handle=pv_name1, secret_name=SECRET_NAME,
             options=[f"subdir={pv_name1}"])
    LOG.info("Deploy pv {}".format(pv1.name))
    pv1.create()

    pv_name2 = "pv-two"
    pv2 = PV(name=pv_name2, access_mode="ReadWriteMany", volume_handle=pv_name2, secret_name=SECRET_NAME,
             options=[f"subdir={pv_name2}"])
    LOG.info("Deploy pv {}".format(pv2.name))
    pv2.create()

    # deploy pvc
    pvc1 = PVC(name="pvc-one", access_mode="ReadWriteMany", storage_name="", pv=pv1.name)
    LOG.info("Deploy pvc {}".format(pvc1.name))
    pvc1.create()
    pvc2 = PVC(name="pvc-two", access_mode="ReadWriteMany", storage_name="", pv=pv2.name)
    LOG.info("Deploy pvc {}".format(pvc2.name))
    pvc2.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc1.check_is_bound() and pvc2.check_is_bound():
            break
        time.sleep(1)

    output = gen_random_string(6) + ".txt"
    # deploy pod
    deployment = Deployment(name="deploy-two", pvc="", replicas=1, out_put=output, pvcs=[pvc1.name, pvc2.name])
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=1)
    LOG.info("Watch for pods of {} for success.".format(pod.name))
    result = pod.watch_for_success()
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            if not check_pod_ready(po):
                subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point for pvc1
    LOG.info("Check mount point..")
    volume_id = pvc1.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/" + output
    result = check_mount_point(check_path)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    volume_id = pvc2.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/" + output
    result = check_mount_point(check_path)
    if not result:
        pods = client.CoreV1Api().list_namespaced_pod(
            namespace="default",
            label_selector="deployment={}".format(deployment.name)
        )
        for po in pods.items:
            pod_name = po.metadata.name
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
            subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))

    # check sidecar
    LOG.info("Check sidecar..")
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    if len(pods.items) != 1:
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))
    pod = pods.items[0]
    if len(pod.spec.containers) != 3:
        raise Exception(
            "Pod {} should have 3 containers, only {} has been found".format(pod.name, len(pod.spec.containers)))
    LOG.info("Test pass.")

    # delete test resources
    LOG.info("Remove deployment {}".format(deployment.name))
    deployment.delete()
    LOG.info("Remove pvc 1 {}".format(pvc1.name))
    pvc1.delete()
    LOG.info("Remove pvc 2 {}".format(pvc2.name))
    pvc2.delete()
    return


def test_dynamic_expand():
    if not is_quota_supported():
        LOG.info("juicefs donot support quota, skip.")
        return
    LOG.info("[test case] Dynamic PVC capacity expand begin..")
    # deploy pvc
    pvc = PVC(name="pvc-cap-expand", access_mode="ReadWriteMany", storage_name=STORAGECLASS_NAME, pv="")
    LOG.info("Deploy pvc {}".format(pvc.name))
    pvc.create()

    # wait for pvc bound
    for i in range(0, 60):
        if pvc.check_is_bound():
            break
        time.sleep(1)

    # deploy pod
    deployment = Deployment(name="app-dynamic-cap-expand", pvc=pvc.name, replicas=1)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                if not check_pod_ready(po):
                    subprocess.check_call(["kubectl", "get", "po", pod_name, "-o", "yaml", "-n", "default"])
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
        if MOUNT_MODE == "webhook":
            pods = client.CoreV1Api().list_namespaced_pod(
                namespace="default",
                label_selector="deployment={}".format(deployment.name)
            )
            for po in pods.items:
                pod_name = po.metadata.name
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "jfs-mount", "-n", "default"])
                subprocess.check_call(["kubectl", "logs", pod_name, "-c", "app", "-n", "default"])
        raise Exception("mount Point of /jfs/{}/out.txt are not ready within 5 min.".format(volume_id))
    LOG.info("Test pass.")

    # expand pvc
    LOG.info("Expand pvc {} to 2Gi".format(pvc.name))
    pvc.update_capacity("2Gi")
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace="default",
        label_selector="deployment={}".format(deployment.name)
    )
    check_quota(pods.items[0].metadata.name, "2.0G")

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
