#!/bin/bash
#
# Copyright 2022 Juicedata Inc
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

tmp_file="webhook.yaml"
tmp_install_script="webhook.sh.bak"
install_script="scripts/webhook.sh"

cat deploy/webhook.yaml >> $tmp_file

let start_num=$(cat -n $install_script | grep "# webhook.yaml start" | awk '{print $1}')+1
let end_num=$(cat -n $install_script | grep "# webhook.yaml end" | awk '{print $1}')-1

head -$start_num $install_script >$tmp_install_script
cat $tmp_file >>$tmp_install_script
tail -n +$end_num $install_script >>$tmp_install_script

mv $tmp_install_script $install_script
chmod +x $install_script

rm -rf $tmp_file
rm -rf $tmp_file.bak
