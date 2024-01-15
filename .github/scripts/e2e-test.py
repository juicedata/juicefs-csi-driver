#  Copyright 2023 Juicedata Inc
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

from kubernetes import config

from config import GLOBAL_MOUNTPOINT, LOG, IN_CCI, IS_CE
from test_case import (
    test_dynamic_mount_image_with_webhook,
    test_static_mount_image_with_webhook,
    test_deployment_dynamic_patch_pv_with_webhook,
    test_deployment_static_patch_pv_with_webhook,
    test_job_complete_using_storage,
    test_static_job_complete,
    test_static_delete_policy,
    test_deployment_using_storage_rw,
    test_quota_using_storage_rw,
    test_deployment_use_pv_rw,
    test_deployment_use_pv_ro,
    test_delete_all,
    test_delete_one,
    test_delete_pvc,
    test_deployment_dynamic_patch_pv,
    test_dynamic_delete_pod,
    test_static_delete_pod,
    test_deployment_static_patch_pv,
    test_dynamic_cache_clean_upon_umount,
    test_static_cache_clean_upon_umount,
    test_dynamic_mount_image,
    test_static_mount_image,
    test_pod_resource_err,
    test_cache_client_conf,
    test_share_mount,
    test_path_pattern_in_storage_class,
    test_dynamic_pvc_delete_with_path_pattern,
    test_dynamic_pvc_delete_not_last_with_path_pattern,
    test_webhook_two_volume,
    test_dynamic_expand,
    test_multi_pvc,
)
from util import die, mount_on_host, umount, clean_juicefs_volume, deploy_secret_and_sc, check_do_test

if __name__ == "__main__":
    test_mode = os.getenv("TEST_MODE")
    without_kubelet = os.getenv("WITHOUT_KUBELET") == "true"
    if check_do_test():
        config.load_kube_config()
        # clear juicefs volume first.
        LOG.info("clean juicefs volume first.")
        mount_on_host(GLOBAL_MOUNTPOINT)
        clean_juicefs_volume()
        try:
            deploy_secret_and_sc()

            if test_mode == "pod":
                test_static_cache_clean_upon_umount()
                test_dynamic_cache_clean_upon_umount()
                test_static_delete_policy()
                test_deployment_using_storage_rw()
                test_deployment_use_pv_rw()
                test_deployment_use_pv_ro()
                test_delete_one()
                test_delete_all()
                test_delete_pvc()
                test_dynamic_delete_pod()
                test_static_delete_pod()
                test_deployment_dynamic_patch_pv()
                test_deployment_static_patch_pv()
                test_dynamic_mount_image()
                test_static_mount_image()
                test_quota_using_storage_rw()
                test_dynamic_expand()
                test_multi_pvc()
                if without_kubelet:
                    test_pod_resource_err()

            elif test_mode == "pod-mount-share":
                if not IS_CE:
                    test_cache_client_conf()

                test_static_cache_clean_upon_umount()
                test_dynamic_cache_clean_upon_umount()
                test_deployment_using_storage_rw()
                test_deployment_use_pv_rw()
                test_deployment_use_pv_ro()
                test_static_delete_policy()
                test_delete_pvc()
                test_share_mount()
                test_delete_one()
                test_delete_all()
                test_dynamic_delete_pod()
                test_static_delete_pod()
                test_deployment_dynamic_patch_pv()
                test_deployment_static_patch_pv()
                test_dynamic_mount_image()
                test_static_mount_image()
                test_quota_using_storage_rw()
                test_dynamic_expand()
                test_multi_pvc()
                if without_kubelet:
                    test_pod_resource_err()

            elif test_mode == "pod-provisioner":
                test_static_cache_clean_upon_umount()
                test_dynamic_cache_clean_upon_umount()
                test_deployment_using_storage_rw()
                test_deployment_use_pv_rw()
                test_deployment_use_pv_ro()
                test_static_delete_policy()
                test_delete_pvc()
                test_deployment_dynamic_patch_pv()
                test_deployment_static_patch_pv()
                test_dynamic_mount_image()
                test_static_mount_image()
                test_path_pattern_in_storage_class()
                test_dynamic_pvc_delete_with_path_pattern()
                test_dynamic_pvc_delete_not_last_with_path_pattern()
                test_delete_one()
                test_delete_all()
                test_dynamic_delete_pod()
                test_static_delete_pod()
                test_quota_using_storage_rw()
                test_dynamic_expand()
                test_multi_pvc()
                if without_kubelet:
                    test_pod_resource_err()

            elif test_mode == "webhook":
                test_deployment_use_pv_rw()
                test_deployment_use_pv_ro()
                test_webhook_two_volume()
                test_static_delete_policy()
                test_static_mount_image_with_webhook()
                test_deployment_static_patch_pv_with_webhook()
                test_static_job_complete()
                if not IN_CCI:
                    test_delete_pvc()
                    test_job_complete_using_storage()
                    test_deployment_using_storage_rw()
                    test_dynamic_mount_image_with_webhook()
                    test_deployment_dynamic_patch_pv_with_webhook()
                    test_quota_using_storage_rw()
                    test_dynamic_expand()


            elif test_mode == "webhook-provisioner":
                test_webhook_two_volume()
                test_static_delete_policy()
                test_deployment_use_pv_rw()
                test_deployment_use_pv_ro()
                test_deployment_static_patch_pv_with_webhook()
                test_static_mount_image_with_webhook()
                test_static_job_complete()
                if not IN_CCI:
                    test_delete_pvc()
                    test_deployment_using_storage_rw()
                    test_deployment_dynamic_patch_pv_with_webhook()
                    test_dynamic_mount_image_with_webhook()
                    test_path_pattern_in_storage_class()
                    test_dynamic_pvc_delete_with_path_pattern()
                    test_dynamic_pvc_delete_not_last_with_path_pattern()
                    test_job_complete_using_storage()
                    test_quota_using_storage_rw()
                    test_dynamic_expand()

            elif test_mode == "process":
                test_static_delete_policy()
                test_static_cache_clean_upon_umount()
                test_dynamic_cache_clean_upon_umount()
                test_deployment_using_storage_rw()
                test_deployment_use_pv_rw()
                test_deployment_use_pv_ro()
                test_delete_pvc()
                test_quota_using_storage_rw()
                test_dynamic_expand()
            else:
                raise Exception("unknown test mode: %s" % test_mode)
        except Exception as e:
            die(e)
        finally:
            # tear_down()
            umount(GLOBAL_MOUNTPOINT)
    else:
        LOG.info("skip test.")
