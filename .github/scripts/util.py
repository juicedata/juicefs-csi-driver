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

import hashlib
import os
import random
import re
import string
import subprocess
import time
import yaml
from pathlib import Path

from kubernetes import client

from config import KUBE_SYSTEM, LOG, IS_CE, SECRET_NAME, GLOBAL_MOUNTPOINT, SECRET_KEY, ACCESS_KEY, META_URL, \
    BUCKET, TOKEN, STORAGECLASS_NAME, CONFIG_NAME
from model import Pod, Secret, STORAGE, StorageClass, PODS, DEPLOYMENTs, PVCs, PVs, SECRETs, STORAGECLASSs


def check_do_test():
    if IS_CE:
        return True
    if TOKEN == "":
        return False
    return True


def die(e):
    csi_node_name = os.getenv("JUICEFS_CSI_NODE_POD")
    if csi_node_name is not None:
        po = Pod(name=csi_node_name, deployment_name="", replicas=1, namespace=KUBE_SYSTEM)
        LOG.info("Get csi node log:")
        LOG.info(po.get_log("juicefs-plugin"))
    LOG.info("Get csi controller log:")
    controller_po = Pod(name="juicefs-csi-controller-0", deployment_name="", replicas=1, namespace=KUBE_SYSTEM)
    LOG.info(controller_po.get_log("juicefs-plugin"))
    LOG.info("Get event: ")
    subprocess.run(["sudo", "kubectl", "get", "event", "--all-namespaces"], check=True)
    LOG.info("Get pvc: ")
    subprocess.run(["sudo", "kubectl", "get", "pvc", "--all-namespaces"], check=True)
    LOG.info("Get pv: ")
    subprocess.run(["sudo", "kubectl", "get", "pv"], check=True)
    LOG.info("Get sc: ")
    subprocess.run(["sudo", "kubectl", "get", "sc"], check=True)
    LOG.info("Get job: ")
    subprocess.run(["sudo", "kubectl", "get", "job", "--all-namespaces"], check=True)
    raise Exception(e)


def mount_on_host(mount_path):
    LOG.info(f"Mount {mount_path}")
    try:
        if IS_CE:
            subprocess.run(
                ["sudo", "/usr/local/bin/juicefs", "format", f"--storage={STORAGE}", f"--access-key={ACCESS_KEY}",
                 f"--secret-key={SECRET_KEY}", f"--bucket={BUCKET}", META_URL, SECRET_NAME],
                check=True
            )
            subprocess.run(
                ["sudo", "/usr/local/bin/juicefs", "mount", "-d", META_URL, mount_path],
                check=True
            )
        else:
            subprocess.run(
                ["sudo", "/usr/bin/juicefs", "auth", f"--token={TOKEN}", f"--access-key={ACCESS_KEY}",
                 f"--secret-key={SECRET_KEY}", f"--bucket={BUCKET}", SECRET_NAME],
                check=True
            )
            subprocess.run(
                ["sudo", "/usr/bin/juicefs", "mount", "-d", SECRET_NAME, mount_path],
                check=True
            )
        LOG.info("Mount success.")
    except Exception as e:
        LOG.info("Error in juicefs mount: {}".format(e))
        raise e


def umount(mount_path):
    subprocess.check_call(["sudo", "umount", mount_path, "-l"])


def check_mount_point(check_path):
    check_path = GLOBAL_MOUNTPOINT + "/" + check_path
    for i in range(0, 60):
        try:
            LOG.info("Open file {}".format(check_path))
            with open(check_path) as f:
                content = f.read(1)
                if content is not None and content != "":
                    return True
                time.sleep(5)
        except FileNotFoundError:
            LOG.info(os.listdir(GLOBAL_MOUNTPOINT))
            LOG.info("Can't find file: {}".format(check_path))
            time.sleep(5)
            continue
        except Exception as e:
            LOG.info(e)
            log = open("/var/log/juicefs.log", "rt")
            LOG.info(log.read())
            raise e
    return False


def check_quota(name, expected):
    output = ""
    for i in range(0, 30):
        process = subprocess.run([
            "kubectl", "exec", name, "-c", "app", "-n", "default", "-t", "--", "df", "-h"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True)
        if process.returncode is not None and process.returncode != 0:
            raise Exception("df -h failed: {}".format(process.stderr))
        output = process.stdout
        quota = None
        for line in process.stdout.split("\n"):
            if line.startswith("JuiceFS:"):
                items = line.split()
                if len(items) >= 2:
                    quota = items[1]
        if quota is None:
            raise Exception("df -h result does not contain juicefs info:\n{}".format(process.stdout))
        if quota != expected:
            time.sleep(1)
            continue
        LOG.info("df -h result: {}".format(process.stdout))
        return
    raise Exception("quota is not set:\n{}".format(output))

def check_quota_in_host(subpath, expected):
    output = ""
    for i in range(0, 30):
        process = subprocess.run(["df", "-h", GLOBAL_MOUNTPOINT+ "/" + subpath],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True)
        if process.returncode is not None and process.returncode != 0:
            LOG.info("df -h failed: {}".format(process.stderr))
            time.sleep(1)
            continue
        output = process.stdout
        quota = None
        for line in process.stdout.split("\n"):
            if line.startswith("JuiceFS:"):
                items = line.split()
                if len(items) >= 2:
                    quota = items[1]
        if quota is None:
            raise Exception("df -h result does not contain juicefs info:\n{}".format(process.stdout))
        if quota != expected:
            time.sleep(1)
            continue
        LOG.info("df -h result: {}".format(process.stdout))
        return
    raise Exception("quota is not set:\n{}".format(output))

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
            time.sleep(5)
            continue
        if output.stdout.decode("utf-8") != "":
            return True
        time.sleep(5)
    return False


def get_only_mount_pod_name(volume_id):
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace=KUBE_SYSTEM,
        label_selector="volume-id={}".format(volume_id)
    )
    running_pods = []
    for pod in pods.items:
        if pod.metadata.deletion_timestamp is None:
            running_pods.append(pod)
    if len(running_pods) == 0:
        raise Exception("Can't get mount pod of volume id {}".format(volume_id))
    if len(running_pods) > 1:
        raise Exception("Get more than one mount pod of volume id {}".format(volume_id))
    return running_pods[0].metadata.name


def wait_get_only_mount_pod_name(volume_id, timeout=60):
    for i in range(0, timeout):
        try:
            return get_only_mount_pod_name(volume_id)
        except Exception as e:
            time.sleep(1)
            continue


def get_mount_pods(volume_id):
    pods = client.CoreV1Api().list_namespaced_pod(
        namespace=KUBE_SYSTEM,
        label_selector="volume-id={}".format(volume_id)
    )
    return pods


def get_voldel_job(volume_id):
    hash_object = hashlib.sha256(volume_id.encode('utf-8'))
    hash_value = hash_object.hexdigest()[:16]
    juicefs_hash = f"juicefs-{hash_value}"[:16]
    for i in range(0, 300):
        try:
            job = client.BatchV1Api().read_namespaced_job(
                namespace=KUBE_SYSTEM,
                name=f"{juicefs_hash}-delvol"
            )
            return job
        except client.exceptions.ApiException as e:
            if e.status == 404:
                time.sleep(0.5)
                continue
            raise e


def check_pod_ready(pod):
    if pod.status.phase.lower() != "running":
        LOG.info("Pod {} status phase: {}".format(pod.metadata.name, pod.status.phase))
        return False
    conditions = pod.status.conditions
    for c in conditions:
        if c.status != "True":
            return False
    return True


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
        clean_juicefs_volume()
    except Exception as e:
        LOG.info("Error in tear down: {}".format(e))
    LOG.info("Tear down success.")


def clean_juicefs_volume():
    visible_files = [file for file in Path(GLOBAL_MOUNTPOINT).iterdir() if not file.name.startswith(".")]
    if len(visible_files) != 0:
        if IS_CE:
            subprocess.check_call(["/usr/local/bin/juicefs rmr " + GLOBAL_MOUNTPOINT + "/*"], shell=True)
        else:
            # only delete files out of 3 days
            for file in visible_files:
                try:
                    f_time = file.stat().st_ctime
                    now = time.time()
                    if now - f_time > 3600 * 24 * 3:
                        subprocess.run(["/usr/bin/juicefs", "rmr", str(file)],
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                except FileNotFoundError:
                    continue


def gen_random_string(slen=10):
    return ''.join(random.sample(string.ascii_letters + string.digits, slen))


def get_vol_uuid(name):
    output = subprocess.run(
        ["sudo", "/usr/local/bin/juicefs", "status", name], stdout=subprocess.PIPE)
    out = output.stdout.decode("utf-8")
    return re.search("\"UUID\": \"(.*)\"", out).group(1)


def is_quota_supported():
    if IS_CE:
        output = subprocess.run(["/usr/local/bin/juicefs", "quota"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        out = output.stderr.decode("utf-8")
        if "No help topic for 'quota'" in out:
            return False
    else:
        output = subprocess.run(["/usr/bin/juicefs", "quota"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        out = output.stdout.decode("utf-8")
        if "invalid command: quota" in out:
            return False
    return True


def get_config() -> dict:
    config_map = client.CoreV1Api().read_namespaced_config_map(name=CONFIG_NAME, namespace=KUBE_SYSTEM)
    return yaml.load(config_map.data["config.yaml"], Loader=yaml.FullLoader)


def update_config(data: dict):
    # convert data to yaml
    data = yaml.dump(data)
    client.CoreV1Api().patch_namespaced_config_map(name=CONFIG_NAME, namespace=KUBE_SYSTEM,
                                                   body={"data": {"config.yaml": data}})
