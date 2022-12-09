from kubernetes import config

from config import GLOBAL_MOUNTPOINT, LOG
from test_case import (
    test_static_delete_policy,
    test_deployment_using_storage_rw,
    test_deployment_using_storage_ro,
    test_dynamic_mount_image_with_webhook,
    test_static_mount_image_with_webhook,
    test_deployment_dynamic_patch_pv_with_webhook,
    test_deployment_static_patch_pv_with_webhook,
)
from util import die, mount_on_host, umount, clean_juicefs_volume, deploy_secret_and_sc, tear_down, check_do_test

if __name__ == "__main__":
    if check_do_test():
        config.load_kube_config()
        # clear juicefs volume first.
        LOG.info("clean juicefs volume first.")
        mount_on_host(GLOBAL_MOUNTPOINT)
        clean_juicefs_volume()
        try:
            deploy_secret_and_sc()
            test_static_delete_policy()
            test_deployment_using_storage_rw()
            test_deployment_using_storage_ro()
            test_dynamic_mount_image_with_webhook()
            test_static_mount_image_with_webhook()
            test_deployment_dynamic_patch_pv_with_webhook()
            test_deployment_static_patch_pv_with_webhook()
        except Exception as e:
            die(e)
        finally:
            tear_down()
            umount(GLOBAL_MOUNTPOINT)
    else:
        LOG.info("skip test.")
