/*
 Copyright 2026 Juicedata Inc

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

package passfd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type testDirEntry struct {
	name  string
	isDir bool
}

func (d testDirEntry) Name() string               { return d.name }
func (d testDirEntry) IsDir() bool                { return d.isDir }
func (d testDirEntry) Type() fs.FileMode          { return 0 }
func (d testDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func TestParseFuseFdsContinuesWhenSubdirDisappears(t *testing.T) {
	oldNodeName := config.NodeName
	oldNamespace := config.Namespace
	config.NodeName = "test-node"
	config.Namespace = "default"
	t.Cleanup(func() {
		config.NodeName = oldNodeName
		config.Namespace = oldNamespace
	})

	basePath := t.TempDir()
	const missingUUID = "uuid-missing"
	var entries []os.DirEntry
	var pods []runtime.Object
	for i := 0; i < 25; i++ {
		upgradeUUID := fmt.Sprintf("uuid-%02d", i)
		entries = append(entries, testDirEntry{name: upgradeUUID, isDir: true})
		pods = append(pods, newFusePassPod(upgradeUUID))
	}
	entries = append(entries, testDirEntry{name: missingUUID, isDir: true})

	var activeParsers int32
	patches := ApplyFunc(os.ReadDir, func(name string) ([]os.DirEntry, error) {
		switch {
		case name == basePath:
			return entries, nil
		case filepath.Base(name) == missingUUID:
			return nil, os.ErrNotExist
		default:
			return []os.DirEntry{testDirEntry{name: "fuse_fd_comm.1"}}, nil
		}
	})
	patches.ApplyFunc(GetFuseFd, func(string, bool) (int, []byte) {
		atomic.AddInt32(&activeParsers, 1)
		defer atomic.AddInt32(&activeParsers, -1)
		time.Sleep(10 * time.Millisecond)
		return -1, nil
	})
	defer patches.Reset()

	fds := &Fds{
		client:   &k8sclient.K8sClient{Interface: fake.NewSimpleClientset(pods...)},
		basePath: basePath,
		fds:      map[string]*fd{},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		fds.ParseFuseFds(context.Background())
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ParseFuseFds did not return")
	}
	if got := atomic.LoadInt32(&activeParsers); got != 0 {
		t.Fatalf("ParseFuseFds returned before parsers finished, active parsers: %d", got)
	}
}

func newFusePassPod(upgradeUUID string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      upgradeUUID,
			Namespace: config.Namespace,
			Labels: map[string]string{
				common.PodTypeKey:             common.PodTypeValue,
				common.PodUpgradeUUIDLabelKey: upgradeUUID,
			},
		},
		Spec: corev1.PodSpec{
			NodeName: config.NodeName,
			Containers: []corev1.Container{
				{Image: config.DefaultCEMountImage},
			},
		},
	}
}
