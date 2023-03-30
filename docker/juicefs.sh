#!/bin/bash

ee_mount_path='/sbin/mount.juicefs'
ee_is_running=$(grep -ao "$ee_mount_path" /proc/1/cmdline)
if [[ ! -z "$ee_is_running" ]]
then
  exec /usr/local/bin/juicefs-ee $@
else
  exec /usr/local/bin/juicefs-ce $@
fi
