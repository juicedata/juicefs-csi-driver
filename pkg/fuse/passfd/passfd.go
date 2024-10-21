/*
 Copyright 2023 Juicedata Inc

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
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog/v2"
	k8sMount "k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var fdLog = klog.NewKlogr().WithName("passfd")

type Fds struct {
	client   *k8s.K8sClient
	globalMu sync.Mutex
	basePath string
	fds      map[string]*fd
}

var GlobalFds *Fds

func InitGlobalFds(ctx context.Context, client *k8s.K8sClient, basePath string) error {
	GlobalFds = &Fds{
		globalMu: sync.Mutex{},
		client:   client,
		basePath: basePath,
		fds:      make(map[string]*fd),
	}
	return GlobalFds.ParseFuseFds(ctx)
}

func InitTestFds() {
	GlobalFds = &Fds{
		globalMu: sync.Mutex{},
		basePath: "tmp",
		fds:      make(map[string]*fd),
	}
}

func (fs *Fds) ParseFuseFds(ctx context.Context) error {
	fdLog.V(1).Info("parse fuse fd in basePath", "basePath", fs.basePath)
	var entries []os.DirEntry
	var err error
	err = util.DoWithTimeout(ctx, 2*time.Second, func() error {
		entries, err = os.ReadDir(fs.basePath)
		return err
	})
	if err != nil {
		fdLog.Error(err, "read dir error", "basePath", fs.basePath)
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var subEntries []os.DirEntry
		err = util.DoWithTimeout(ctx, 2*time.Second, func() error {
			subEntries, err = os.ReadDir(path.Join(fs.basePath, entry.Name()))
			return err
		})
		if err != nil {
			fdLog.Error(err, "read dir error", "basePath", fs.basePath)
			return err
		}
		for _, subEntry := range subEntries {
			if strings.HasPrefix(subEntry.Name(), "fuse_fd_comm.") {
				subdir := path.Join(fs.basePath, entry.Name(), subEntry.Name())
				fdLog.V(1).Info("parse fuse fd", "path", subdir)
				fs.parseFuse(ctx, entry.Name(), subdir)
			}
		}
	}
	return nil
}

func (fs *Fds) GetFdAddress(ctx context.Context, podHashVal string) (string, error) {
	if f, ok := fs.fds[podHashVal]; ok {
		return f.serverAddressInPod, nil
	}

	address := path.Join(fs.basePath, podHashVal, "fuse_fd_csi_comm.sock")
	addressInPod := path.Join(fs.basePath, "fuse_fd_csi_comm.sock")
	// mkdir parent
	err := util.DoWithTimeout(ctx, 2*time.Second, func() error {
		parentPath := path.Join(fs.basePath, podHashVal)
		exist, _ := k8sMount.PathExists(parentPath)
		if !exist {
			return os.MkdirAll(parentPath, 0777)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	fs.globalMu.Lock()
	fs.fds[podHashVal] = &fd{
		done:               make(chan struct{}),
		fuseFd:             0,
		fuseSetting:        []byte("FUSE"),
		serverAddress:      address,
		serverAddressInPod: addressInPod,
	}
	fs.globalMu.Unlock()

	return addressInPod, nil
}

func (fs *Fds) StopFd(ctx context.Context, podHashVal string) {
	if podHashVal == "" {
		return
	}
	fs.globalMu.Lock()
	f := fs.fds[podHashVal]
	if f == nil {
		serverParentPath := path.Join(fs.basePath, podHashVal)
		_ = util.DoWithTimeout(ctx, 2*time.Second, func() error {
			_, err := os.Stat(serverParentPath)
			if err == nil {
				_ = os.RemoveAll(serverParentPath)
			}
			return nil
		})
		fs.globalMu.Unlock()
		return
	}
	fdLog.V(1).Info("stop fuse fd server", "server address", f.serverAddress)
	close(f.done)
	delete(fs.fds, podHashVal)

	serverParentPath := path.Join(fs.basePath, podHashVal)
	_ = util.DoWithTimeout(ctx, 2*time.Second, func() error {
		_ = os.RemoveAll(serverParentPath)
		return nil
	})
	fs.globalMu.Unlock()
}

func (fs *Fds) CloseFd(podHashVal string) {
	fs.globalMu.Lock()
	f := fs.fds[podHashVal]
	if f == nil {
		fs.globalMu.Unlock()
		return
	}
	fdLog.V(1).Info("close fuse fd", "hashVal", podHashVal)
	_ = syscall.Close(f.fuseFd)
	f.fuseFd = -1
	fs.fds[podHashVal] = f
	fs.globalMu.Unlock()
}

func (fs *Fds) parseFuse(ctx context.Context, podHashVal, fusePath string) {
	fuseFd, fuseSetting := GetFuseFd(fusePath, false)
	if fuseFd <= 0 {
		// get fuse fd error, try to get mount pod
		labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
			common.PodTypeKey:           common.PodTypeValue,
			common.PodJuiceHashLabelKey: podHashVal,
		}}
		fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
		pods, err := fs.client.ListPod(ctx, config.Namespace, labelSelector, fieldSelector)
		if err != nil {
			fdLog.Error(err, "list pods error")
			return
		}
		var mountPod *corev1.Pod
		for _, pod := range pods {
			if pod.DeletionTimestamp == nil {
				mountPod = &pod
				break
			}
		}
		if mountPod == nil {
			fdLog.V(1).Info("get fuse fd error and mount pod not found, ignore it", "hashVal", podHashVal, "fusePath", fusePath)
			// if can not get fuse fd, do not serve for it
			return
		}
	}

	serverPath := path.Join(fs.basePath, podHashVal, "fuse_fd_csi_comm.sock")
	serverPathInPod := path.Join(fs.basePath, "fuse_fd_csi_comm.sock")
	fdLog.V(1).Info("fuse fd path of pod", "hashVal", podHashVal, "fusePath", fusePath)

	f := &fd{
		done:               make(chan struct{}),
		fuseFd:             0,
		fuseSetting:        []byte("FUSE"),
		serverAddress:      serverPath,
		serverAddressInPod: serverPathInPod,
	}
	f.fuseFd, f.fuseSetting = fuseFd, fuseSetting

	fs.globalMu.Lock()
	fs.fds[podHashVal] = f
	fs.globalMu.Unlock()

	fs.serveFuseFD(ctx, podHashVal)
}

type fd struct {
	done chan struct{}

	fuseFd      int
	fuseSetting []byte
	sid         uint64

	serverAddress      string // server for pod
	serverAddressInPod string // server path in pod
}

func (fs *Fds) ServeFuseFd(ctx context.Context, podHashVal string) error {
	if _, ok := fs.fds[podHashVal]; ok {
		fs.serveFuseFD(ctx, podHashVal)
		return nil
	}
	return fmt.Errorf("fuse fd of podHashVal %s not found in global fuse fds", podHashVal)
}

func (fs *Fds) serveFuseFD(ctx context.Context, podHashVal string) {
	f := fs.fds[podHashVal]
	if f == nil {
		return
	}

	fdLog.V(1).Info("serve FUSE fd", "fd", f.fuseFd, "server address", f.serverAddress)
	_ = util.DoWithTimeout(ctx, 2*time.Second, func() error {
		_ = os.Remove(f.serverAddress)
		return nil
	})
	sock, err := net.Listen("unix", f.serverAddress)
	if err != nil {
		fdLog.Error(err, "listen unix socket error")
		return
	}
	go func() {
		defer func() {
			_ = util.DoWithTimeout(ctx, 2*time.Second, func() error {
				_ = os.Remove(f.serverAddress)
				return nil
			})
		}()
		defer sock.Close()
		<-f.done
		_ = syscall.Close(f.fuseFd)
	}()
	go func() {
		for {
			conn, err := sock.Accept()
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
				fdLog.Error(err, "accept error")
				continue
			}
			go fs.handleFDRequest(podHashVal, conn.(*net.UnixConn))
		}
	}()
}

func (fs *Fds) handleFDRequest(podHashVal string, conn *net.UnixConn) {
	defer conn.Close()
	f := fs.fds[podHashVal]
	if f == nil {
		return
	}
	var fds = []int{0}
	fs.globalMu.Lock()
	if f.fuseFd > 0 {
		fds = append(fds, f.fuseFd)
		fdLog.V(1).Info("send FUSE fd", "fd", f.fuseFd)
	}
	err := putFd(conn, f.fuseSetting, fds...)
	if err != nil {
		fs.globalMu.Unlock()
		fdLog.Error(err, "send fuse fds error")
		return
	}
	if f.fuseFd > 0 {
		_ = syscall.Close(f.fuseFd)
		f.fuseFd = -1
	}
	fs.globalMu.Unlock()

	var msg []byte
	msg, fds, err = getFd(conn, 1)
	if err != nil {
		fdLog.Error(err, "recv fuse fds")
		return
	}

	fs.globalMu.Lock()
	if string(msg) != "CLOSE" && f.fuseFd <= 0 && len(fds) >= 1 {
		f.fuseFd = fds[0]
		f.fuseSetting = msg
		fdLog.V(1).Info("recv FUSE fd", "fd", fds)
	} else {
		for _, fd := range fds {
			_ = syscall.Close(fd)
		}
		fdLog.V(1).Info("recv msg and fds", "msg", string(msg), "fd", fds)
	}
	fs.fds[podHashVal] = f
	fs.globalMu.Unlock()
}

func (fs *Fds) UpdateSid(podHashVal string, sid uint64) {
	f := fs.fds[podHashVal]
	if f == nil {
		return
	}

	fs.globalMu.Lock()
	f.sid = sid
	fs.fds[podHashVal] = f
	fs.globalMu.Unlock()
}

func (fs *Fds) GetSid(podHashVal string) uint64 {
	f := fs.fds[podHashVal]
	if f == nil {
		return 0
	}

	fs.globalMu.Lock()
	sid := f.sid
	fs.globalMu.Unlock()
	return sid
}

func GetFuseFd(path string, close bool) (int, []byte) {
	var exists bool
	if err := util.DoWithTimeout(context.TODO(), time.Second*3, func() (err error) {
		exists, err = k8sMount.PathExists(path)
		return
	}); err != nil {
		fdLog.V(1).Info("path exists error", "path", path)
		return -1, nil
	}

	if !exists {
		return -1, nil
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		fdLog.V(1).Info("dial error", "path", path, "error", err)
		return -1, nil
	}
	defer conn.Close()
	msg, fds, err := getFd(conn.(*net.UnixConn), 2)
	if err != nil {
		fdLog.Error(err, "recv fds error")
		return -1, nil
	}
	fdLog.V(1).Info("get fd and msg", "fd", fds)
	_ = syscall.Close(fds[0])
	if close {
		fdLog.V(1).Info("send close fuse fd")
		_ = putFd(conn.(*net.UnixConn), []byte("CLOSE"), 0) // close it
		if len(fds) > 1 {
			// close it in csi also
			_ = syscall.Close(fds[1])
			fdLog.Info("fd ")
			return fds[1], msg
		}
		return fds[0], msg
	}
	if len(fds) > 1 {
		fdLog.V(1).Info("send FUSE fd", "fd", fds[1])
		err = putFd(conn.(*net.UnixConn), msg, fds[1])
		if err != nil {
			fdLog.Error(err, "send FUSE error")
		}
		return fds[1], msg
	}
	return 0, nil
}

// GetFd: Get receives file descriptors from a Unix domain socket.
// Num specifies the expected number of file descriptors in one message.
// Internal files' names to be assigned are specified via optional filenames
// argument.
// You need to close all files in the returned slice. The slice can be
// non-empty even if this function returns an error.
func getFd(via *net.UnixConn, num int) ([]byte, []int, error) {
	if num < 1 {
		return nil, nil, nil
	}

	// get the underlying socket
	viaf, err := via.File()
	if err != nil {
		return nil, nil, err
	}
	defer viaf.Close()
	socket := int(viaf.Fd())

	// recvmsg
	msg := make([]byte, syscall.CmsgSpace(100))
	oob := make([]byte, syscall.CmsgSpace(num*4))
	n, oobn, _, _, err := syscall.Recvmsg(socket, msg, oob, 0)
	if err != nil {
		return nil, nil, err
	}

	// parse control msgs
	msgs, err := syscall.ParseSocketControlMessage(oob[:oobn])

	// convert fds to files
	fds := make([]int, 0, len(msgs))
	for _, msg := range msgs {
		var rights []int
		rights, err = syscall.ParseUnixRights(&msg)
		fds = append(fds, rights...)
		if err != nil {
			for i := range fds {
				syscall.Close(fds[i])
			}
			fds = nil
			break
		}
	}
	return msg[:n], fds, err
}

// putFd sends file descriptors to Unix domain socket.
//
// Please note that the number of descriptors in one message is limited
// and is rather small.
func putFd(via *net.UnixConn, msg []byte, fds ...int) error {
	if len(fds) == 0 {
		return nil
	}
	viaf, err := via.File()
	if err != nil {
		return err
	}
	defer viaf.Close()
	socket := int(viaf.Fd())
	rights := syscall.UnixRights(fds...)
	return syscall.Sendmsg(socket, msg, rights, nil, 0)
}
