from kubernetes import config

from config import LOG, GLOBAL_MOUNTPOINT
from test_case import (
    test_deployment_using_storage_ro,
    test_deployment_using_storage_rw,
    test_deployment_use_pv_ro,
    test_delete_pvc,
    test_static_cache_clean_upon_umount,
    test_dynamic_cache_clean_upon_umount,
    test_deployment_use_pv_rw
)
from util import die, mount_on_host, clean_juicefs_volume, deploy_secret_and_sc, tear_down, check_do_test, umount

if __name__ == "__main__":
    if check_do_test():
        config.load_kube_config()
        # clear juicefs volume first.
        mount_on_host(GLOBAL_MOUNTPOINT)
        clean_juicefs_volume()
        try:
            deploy_secret_and_sc()
            test_deployment_using_storage_rw()
            test_deployment_using_storage_ro()
            test_deployment_use_pv_rw()
            test_deployment_use_pv_ro()
            test_delete_pvc()
            test_static_cache_clean_upon_umount()
            test_dynamic_cache_clean_upon_umount()
        except Exception as e:
            die(e)
        finally:
            tear_down()
            umount(GLOBAL_MOUNTPOINT)
    else:
        LOG.info("skip test.")
