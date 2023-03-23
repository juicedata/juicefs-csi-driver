/*
Copyright 2021 Juicedata Inc

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

package driver

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetVersionJSON(t *testing.T) {
	Convey("Test GetVersionJSON", t, func() {
		Convey("test normal", func() {
			patch1 := ApplyFunc(GetVersion, func() VersionInfo {
				return VersionInfo{
					DriverVersion: "v0.1.0",
				}
			})
			defer patch1.Reset()
			_, err := GetVersionJSON()
			So(err, ShouldBeNil)
		})
		Convey("MarshalIndent error", func() {
			patch1 := ApplyFunc(json.MarshalIndent, func(v interface{}, prefix, indent string) ([]byte, error) {
				return []byte{}, errors.New("test")
			})
			defer patch1.Reset()
			_, err := GetVersionJSON()
			So(err, ShouldNotBeNil)
		})
	})
}

func TestGetVersion(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		if got := GetVersion(); !strings.Contains(got.GoVersion, "go1.") {
			t.Errorf("GetVersion() = %v, want %v", got, "go1.x")
		}
	})
}
