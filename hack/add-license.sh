# Copyright 2024 Juicedata Inc
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

set -x
set -e
set -o pipefail
set -o nounset

if [[ -z "${1-}" ]]; then
    echo "Usage: $0 <mode>"
    echo "Example: $0 check"
    exit 1
fi

if [[ $1 == "check" || $1 == "run" ]]; then
    mode=$1
else
    echo "Error: mode must be check or run"
    exit 1
fi

args=(
    -c "Juicedata Inc"
    -f LICENSE_TEMPLATE
    -ignore "site/**/*"
    -ignore "**/*.md"
    -ignore "**/*.json"
    -ignore "**/*.yml"
    -ignore "**/*.yaml"
    -ignore "**/*.xml"
    -ignore "**/*.html"
    -ignore "**/node_modules/**"
    -ignore "**/dist"
    -ignore "deploy/**"
    -ignore "docker/**"
    -ignore "scripts/**"
    -ignore ".github/**"
)
if [[ $mode == "check" ]]; then
    args+=(-check)
    if ! addlicense "${args[@]}" .; then
        set +x
        echo -e "\n------------------------------------------------------------------------"
        echo "Error: license missing in one or more files. Run \`$0 run\` to update them."
        exit 1
    fi
    exit 0
fi

addlicense "${args[@]}" .
