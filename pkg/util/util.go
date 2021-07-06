/*
Copyright 2018 The Kubernetes Authors.

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

import (
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func ParseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	addr := path.Join(u.Host, filepath.FromSlash(u.Path))

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "tcp":
	case "unix":
		addr = path.Join("/", addr)
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("could not remove unix domain socket %q: %v", addr, err)
		}
	default:
		return "", "", fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return scheme, addr, nil
}

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func PathExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return false
}

func Copy(fromPath, toPath string) error {
	if !PathExist(fromPath) {
		return status.Errorf(codes.NotFound, "Path %v not found.", fromPath)
	}

	baseToPath := ""
	if strings.HasSuffix(toPath, "/") {
		baseToPath = toPath
	} else {
		pathStr := strings.Split(toPath, "/")
		baseToPath = strings.Join(pathStr[:len(pathStr)-1], "/")
	}
	if !PathExist(baseToPath) {
		err := os.MkdirAll(baseToPath, os.FileMode(0755))
		if err != nil {
			log.Fatal(err)
		}
	}

	exec := k8sexec.New()
	args := []string{fromPath, toPath}
	out, err := exec.Command("cp", args...).CombinedOutput()
	klog.V(5).Infof("cp output: '%s'", out)
	if err != nil {
		return status.Errorf(codes.Internal, "Can't cp %s %s: %v", fromPath, toPath, err)
	}

	return nil
}
