from kubernetes import config

from config import GLOBAL_MOUNTPOINT, LOG
from test_case import (
    test_static_delete_policy,
    test_deployment_using_storage_rw,
    test_deployment_using_storage_ro,
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
    test_static_cache_clean_upon_umount
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
            # test_static_delete_policy()
            # test_deployment_using_storage_rw()
            # test_deployment_using_storage_ro()
            # test_deployment_use_pv_rw()
            # test_deployment_use_pv_ro()
            # test_delete_one()
            # test_delete_all()
            # test_delete_pvc()
            # test_dynamic_delete_pod()
            # test_static_delete_pod()
            # test_static_cache_clean_upon_umount()
            # test_dynamic_cache_clean_upon_umount()
            # test_deployment_dynamic_patch_pv()
            # test_deployment_static_patch_pv()
        except Exception as e:
            die(e)
        finally:
            tear_down()
            umount(GLOBAL_MOUNTPOINT)
    else:
        LOG.info("skip test.")
