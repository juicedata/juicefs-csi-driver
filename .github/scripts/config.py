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

import logging
import os

KUBE_SYSTEM = "default"
META_URL = os.getenv("JUICEFS_META_URL") or ""
ACCESS_KEY = os.getenv("JUICEFS_ACCESS_KEY") or ""
SECRET_KEY = os.getenv("JUICEFS_SECRET_KEY") or ""
STORAGE = os.getenv("JUICEFS_STORAGE") or ""
BUCKET = os.getenv("JUICEFS_BUCKET") or ""
TOKEN = os.getenv("JUICEFS_TOKEN") or ""
JUICEFS_MODE = os.getenv("JUICEFS_MODE")
IS_CE = os.getenv("JUICEFS_MODE") == "ce"
MOUNT_MODE = os.getenv("MOUNT_MODE")
RESOURCE_PREFIX = "{}-{}-".format(MOUNT_MODE, JUICEFS_MODE)

GLOBAL_MOUNTPOINT = "/mnt/jfs"
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
