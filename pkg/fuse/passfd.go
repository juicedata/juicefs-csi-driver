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

package fuse

import (
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"

	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type Fds struct {
	globalMu sync.Mutex
	basePath string
	fds      map[string]*fd
}

var GlobalFds *Fds

func InitGlobalFds(basePath string) {
	GlobalFds = &Fds{
		globalMu: sync.Mutex{},
		basePath: basePath,
		fds:      make(map[string]*fd),
	}
	if err := GlobalFds.ParseFuseFds(basePath); err != nil {
		return
	}
}

func InitTestFds() {
	GlobalFds = &Fds{
		globalMu: sync.Mutex{},
		fds:      make(map[string]*fd),
	}
}

func (fs *Fds) ParseFuseFds(basePath string) error {
	klog.V(6).Infof("parse fuse fd in basePath %s", basePath)
	entries, err := os.ReadDir(basePath)
	if err != nil {
		klog.Errorf("read dir %s: %s", basePath, err)
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subEntries, err := os.ReadDir(path.Join(basePath, entry.Name()))
		if err != nil {
			klog.Errorf("read dir %s: %s", basePath, err)
			return err
		}
		for _, subEntry := range subEntries {
			if strings.HasPrefix(subEntry.Name(), "fuse_fd_comm.") {
				klog.V(6).Infof("parse fuse fd in %s", subEntry.Name())
				fs.parseFuse(entry.Name(), path.Join(basePath, entry.Name(), subEntry.Name()))
			}
		}
	}
	return nil
}

func (fs *Fds) GetFdAddress(podHashVal string) string {
	if f, ok := fs.fds[podHashVal]; ok {
		return f.serverAddressInPod
	}

	address := path.Join("/tmp", podHashVal, "fuse_fd_csi_comm.sock")
	addressInPod := path.Join("/tmp", "fuse_fd_csi_comm.sock")
	// mkdir parent
	_ = os.MkdirAll(path.Join("/tmp", podHashVal), 0777)
	fs.globalMu.Lock()
	fs.fds[podHashVal] = &fd{
		fuseMu:             sync.Mutex{},
		done:               make(chan struct{}),
		fuseFd:             0,
		fuseSetting:        []byte("FUSE"),
		serverAddress:      address,
		serverAddressInPod: addressInPod,
	}
	fs.globalMu.Unlock()

	return addressInPod
}

func (fs *Fds) StopFd(podHashVal string) {
	fs.globalMu.Lock()
	f := fs.fds[podHashVal]
	if f == nil {
		fs.globalMu.Unlock()
		return
	}
	klog.V(6).Infof("stop fuse fd server: %s", f.serverAddress)
	close(f.done)
	delete(fs.fds, podHashVal)

	serverParentPath := path.Join("/tmp", podHashVal)
	_ = os.RemoveAll(serverParentPath)
	fs.globalMu.Unlock()
}

func (fs *Fds) parseFuse(podHashVal, fusePath string) {
	fuseFd, fuseSetting := getFuseFd(fusePath)
	if fuseFd == 0 {
		return
	}

	serverPath := path.Join("/tmp", podHashVal, "fuse_fd_csi_comm.sock")
	serverPathInPod := path.Join("/tmp", "fuse_fd_csi_comm.sock")
	klog.V(6).Infof("fuse fd path of pod %s: %s", podHashVal, fusePath)

	f := &fd{
		fuseMu:             sync.Mutex{},
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

	fs.serveFuseFD(podHashVal)
}

type fd struct {
	fuseMu sync.Mutex
	done   chan struct{}

	fuseFd      int
	fuseSetting []byte

	serverAddress      string // server for pod
	serverAddressInPod string // server path in pod
}

func (fs *Fds) ServeFuseFd(podHashVal string) error {
	if _, ok := fs.fds[podHashVal]; ok {
		fs.serveFuseFD(podHashVal)
		return nil
	}
	return fmt.Errorf("fuse fd of podHashVal %s not found in global fuse fds", podHashVal)
}

func (fs *Fds) serveFuseFD(podHashVal string) {
	f := fs.fds[podHashVal]
	if f == nil {
		return
	}

	klog.Infof("serve fuse fd: %v, path: %s", f.fuseFd, f.serverAddress)
	_ = os.Remove(f.serverAddress)
	sock, err := net.Listen("unix", f.serverAddress)
	if err != nil {
		klog.Error(err)
		return
	}
	go func() {
		defer os.Remove(f.serverAddress)
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
				klog.Warningf("accept : %s", err)
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
	f.fuseMu.Lock()
	if f.fuseFd > 0 {
		fds = append(fds, f.fuseFd)
		klog.V(6).Infof("send FUSE fd: %d", f.fuseFd)
	}
	err := putFd(conn, f.fuseSetting, fds...)
	if err != nil {
		f.fuseMu.Unlock()
		klog.Warningf("send fuse fds: %s", err)
		return
	}
	if f.fuseFd > 0 {
		_ = syscall.Close(f.fuseFd)
		f.fuseFd = -1
	}
	f.fuseMu.Unlock()

	var msg []byte
	msg, fds, err = getFd(conn, 1)
	if err != nil {
		klog.Warningf("recv fuse fds: %s", err)
		return
	}

	f.fuseMu.Lock()
	if string(msg) != "CLOSE" && f.fuseFd <= 0 && len(fds) >= 1 {
		f.fuseFd = fds[0]
		f.fuseSetting = msg
		klog.V(6).Infof("recv FUSE fd: %v", fds)
	} else {
		for _, fd := range fds {
			_ = syscall.Close(fd)
		}
		klog.V(6).Infof("msg: %s fds: %+v", string(msg), fds)
	}
	f.fuseMu.Unlock()

	fs.globalMu.Lock()
	fs.fds[podHashVal] = f
	fs.globalMu.Unlock()
}

func getFuseFd(path string) (int, []byte) {
	if !util.Exists(path) {
		return -1, nil
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		klog.Warningf("dial %s: %s", path, err)
		return -1, nil
	}
	defer conn.Close()
	msg, fds, err := getFd(conn.(*net.UnixConn), 2)
	if err != nil {
		klog.Warningf("recv fds: %s", err)
		return -1, nil
	}
	klog.V(6).Infof("get fd: %v, msg: %v", fds, string(msg))
	_ = syscall.Close(fds[0])
	if len(fds) > 1 {
		err = putFd(conn.(*net.UnixConn), msg, fds[1])
		klog.V(6).Infof("send FUSE: %d", fds[1])
		if err != nil {
			klog.Warningf("send FUSE: %s", err)
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
