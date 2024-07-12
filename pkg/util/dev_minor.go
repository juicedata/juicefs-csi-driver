/*
 Copyright 2024 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package util

import "sync"

var (
	MountPointDevMinorTable sync.Map
)

func DevMinorTableStore(k any, dev uint64) {
	MountPointDevMinorTable.Store(k, DevMinor(dev))
}

func DevMinorTableDelete(k any) {
	MountPointDevMinorTable.Delete(k)
}

func DevMinorTableLoad(k any) (uint64, bool) {
	v, ok := MountPointDevMinorTable.Load(k)
	return v.(uint64), ok
}

// DevMinor returns the minor component of a Linux device number.
func DevMinor(dev uint64) uint32 {
	minor := dev & 0xff
	minor |= (dev >> 12) & 0xffffff00
	return uint32(minor)
}
