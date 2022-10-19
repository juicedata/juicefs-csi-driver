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

from kubernetes import config

from config import GLOBAL_MOUNTPOINT, LOG
from test_case import (
    test_deployment_using_storage_rw,
    test_deployment_use_pv_rw,
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
            test_deployment_using_storage_rw()
            test_deployment_use_pv_rw()
        except Exception as e:
            die(e)
        finally:
            tear_down()
            umount(GLOBAL_MOUNTPOINT)
    else:
        LOG.info("skip test.")
