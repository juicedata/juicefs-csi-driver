package driver

import (
	"encoding/json"
	"errors"
	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
	"runtime"
	"testing"
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
	Convey("Test GetVersion", t, func() {
		Convey("test normal", func() {
			patch1 := ApplyFunc(runtime.Version, func() string {
				return "test"
			})
			defer patch1.Reset()

			got := GetVersion()
			So(got.GoVersion, ShouldEqual, "test")
		})
	})
}
