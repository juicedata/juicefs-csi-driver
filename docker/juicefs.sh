#!/bin/bash

cmdline=$(cat /proc/1/cmdline | tr -d '\0')
any_mount_running=$(echo $cmdline | grep -ao '/mount.juicefs')
if [[ -z "$any_mount_running" ]]
then
  echo 'Cannot infer juicefs client from PID 1, use the following instead:'
  echo '/usr/local/bin/juicefs-ce'
  echo '/usr/local/bin/juicefs-ee'
  exit 0
fi
ee_mount_path='/sbin/mount.juicefs'
ee_is_running=$(echo $cmdline | grep -ao "$ee_mount_path")
if [[ ! -z "$ee_is_running" ]]
then
  exec /usr/local/bin/juicefs-ee $@
else
  exec /usr/local/bin/juicefs-ce $@
fi
