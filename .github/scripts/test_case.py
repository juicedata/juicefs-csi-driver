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
    LOG, PVs, META_URL, MOUNT_MODE
from model import PVC, PV, Pod, StorageClass, Deployment
from util import check_mount_point, wait_dir_empty, wait_dir_not_empty, \
    get_only_mount_pod_name, get_mount_pods, check_pod_ready, check_mount_pod_refs, gen_random_string, get_vol_uuid


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
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    volume_id = pvc.get_volume_id()
    LOG.info("Get volume_id {}".format(volume_id))
    check_path = volume_id + "/out.txt"
    result = check_mount_point(check_path)
    if not result:
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

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-static-rw", pvc=pvc.name, replicas=1, out_put=out_put)
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
    mount_pod_name = get_only_mount_pod_name(volume_id)
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
    mount_pod_name = get_only_mount_pod_name(volume_id)
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
            mount_pod_name = get_only_mount_pod_name(volume_id)
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
            mount_pod_name = get_only_mount_pod_name(volume_id)
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
    subdir = "aaa"
    pv_name = volume_id
    pv = client.CoreV1Api().read_persistent_volume(name=pv_name)
    pv.spec.mount_options = ["subdir={}".format(subdir), "verbose"]
    pv.spec.mount_options.append("subdir={}".format(subdir))
    LOG.info(f"Patch PV {pv_name}: add subdir={subdir} and verbose in mountOptions")
    client.CoreV1Api().patch_persistent_volume(pv_name, pv)

    # delete one app pod
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace=KUBE_SYSTEM,
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
            namespace=KUBE_SYSTEM,
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
        raise Exception("Pods of deployment {} are not ready within 2 min.".format(deployment.name))

    # check mount pod
    LOG.info("Check 2 mount pods.")
    mount_pods = get_mount_pods(volume_id)
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
        namespace=KUBE_SYSTEM,
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
    subdir = "aaa"
    pv_name = pv.name
    pv = client.CoreV1Api().read_persistent_volume(name=pv_name)
    pv.spec.mount_options.append("subdir={}".format(subdir))
    LOG.info(f"Patch PV {pv_name}: add subdir={subdir} in mountOptions")
    client.CoreV1Api().patch_persistent_volume(pv_name, pv)

    # delete one app pod
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace=KUBE_SYSTEM,
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
            namespace=KUBE_SYSTEM,
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
        namespace=KUBE_SYSTEM,
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
    mount_image = "juicedata/mount:v1.0.0-4.8.0"
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
    LOG.info("Remove sc {}".format(pvc.name))
    sc.delete()
    LOG.info("Test pass.")
    return


def test_static_mount_image():
    LOG.info("[test case] Deployment set mount image in PV begin..")
    mount_image = "juicedata/mount:v1.0.0-4.8.0"
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
    label_value = "def"
    anno_value = "xyz"
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

    # deploy pod
    out_put = gen_random_string(6) + ".txt"
    deployment = Deployment(name="app-path-pattern-dynamic", pvc=pvc.name, replicas=1, out_put=out_put)
    LOG.info("Deploy deployment {}".format(deployment.name))
    deployment.create()
    pod = Pod(name="", deployment_name=deployment.name, replicas=deployment.replicas)
    LOG.info("Watch for pods of {} for success.".format(deployment.name))
    result = pod.watch_for_success()
    if not result:
        raise Exception("Pods of deployment {} are not ready within 10 min.".format(deployment.name))

    # check mount point
    LOG.info("Check mount point..")
    check_path = "{}-{}-{}-{}/{}".format(KUBE_SYSTEM, pvc.name, label_value, anno_value, out_put)
    result = check_mount_point(check_path)
    if not result:
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
