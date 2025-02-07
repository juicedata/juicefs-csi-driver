/*
 Copyright 2025 Juicedata Inc

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

package config

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGenJfsVolUUID(t *testing.T) {
	Convey("Test juicefs status", t, func() {
		Convey("normal", func() {
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(`
2022/05/05 07:16:30.498501 juicefs[284385] <INFO>: Meta address: redis://127.0.0.1/1
2022/05/05 07:16:30.500868 juicefs[284385] <WARNING>: AOF is not enabled, you may lose data if Redis is not shutdown properly.
2022/05/05 07:16:30.501443 juicefs[284385] <INFO>: Ping redis: 536.562µs
{
  "Setting": {
    "Name": "minio",
    "UUID": "e267db92-051d-4214-b1aa-e97bf61bff1a",
    "Storage": "minio",
    "Bucket": "http://10.98.166.242:9000/minio/test2",
    "AccessKey": "minioadmin",
    "SecretKey": "removed",
    "BlockSize": 4096,
    "Compression": "none",
    "Shards": 0,
    "Partitions": 0,
    "Capacity": 0,
    "Inodes": 0,
    "TrashDays": 2
  },
  "Sessions": []
}
`), nil
			})
			defer patch3.Reset()

			setting := &JfsSetting{
				Source: "test",
				Envs:   map[string]string{},
				IsCe:   true,
			}
			uuid, err := GetJfsVolUUID(context.TODO(), setting)
			So(err, ShouldBeNil)
			So(uuid, ShouldEqual, "e267db92-051d-4214-b1aa-e97bf61bff1a")
		})
		Convey("status error", func() {
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), errors.New("test")
			})
			defer patch3.Reset()

			setting := &JfsSetting{
				Source: "test",
				Envs:   map[string]string{},
				IsCe:   true,
			}
			_, err := GetJfsVolUUID(context.TODO(), setting)
			So(err, ShouldNotBeNil)
		})
		Convey("ee", func() {
			setting := &JfsSetting{
				Source: "test",
				Name:   "test",
				Envs:   map[string]string{},
				IsCe:   false,
			}
			uuid, err := GetJfsVolUUID(context.TODO(), setting)
			So(err, ShouldBeNil)
			So(uuid, ShouldEqual, "test")
		})
	})
}
