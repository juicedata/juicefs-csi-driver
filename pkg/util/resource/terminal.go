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

package resource

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/net/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	// (Ctrl-C) to interrupt or terminate a program or process.
	EndOfText = "\u0003"
	// (Ctrl-D) to indicate end-of-file on a terminal.
	EndOfTransmission = "\u0004"
)

type terminalSession struct {
	conn              *websocket.Conn
	sizeCh            chan *remotecommand.TerminalSize
	endOfTransmission string
	lastHeartbeatAt   time.Time
}

func NewTerminalSession(ctx context.Context, conn *websocket.Conn, endOfTransmission string) *terminalSession {
	t := &terminalSession{
		conn:              conn,
		sizeCh:            make(chan *remotecommand.TerminalSize),
		endOfTransmission: endOfTransmission,
		lastHeartbeatAt:   time.Now(),
	}
	go t.checkHeartbeat(ctx)
	return t
}

func (t *terminalSession) Write(p []byte) (int, error) {
	err := websocket.Message.Send(t.conn, string(p))
	return len(p), err
}

func (t *terminalSession) Read(p []byte) (int, error) {
	var msgStr []byte
	var msg struct {
		Rows uint16 `json:"rows"`
		Cols uint16 `json:"cols"`
		Data string `json:"data"`
		Type string `json:"type"`
	}
	err := websocket.Message.Receive(t.conn, &msgStr)
	if err != nil {
		return copy(p, t.endOfTransmission), err
	}
	if err := json.Unmarshal(msgStr, &msg); err != nil {
		return copy(p, t.endOfTransmission), nil
	}
	switch msg.Type {
	case "stdin":
		return copy(p, []byte(msg.Data)), nil
	case "resize":
		select {
		case t.sizeCh <- &remotecommand.TerminalSize{
			Width:  msg.Cols,
			Height: msg.Rows,
		}:
		default:
		}
	case "ping":
		t.lastHeartbeatAt = time.Now()
		_ = websocket.Message.Send(t.conn, "pong")
	default:
		return copy(p, t.endOfTransmission), nil
	}
	return 0, nil
}

func (t *terminalSession) Next() *remotecommand.TerminalSize {
	return <-t.sizeCh
}

func (t *terminalSession) checkHeartbeat(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if time.Since(t.lastHeartbeatAt) > 1*time.Minute {
				resourceLog.Info("Terminal session heartbeat timeout")
				t.conn.Close()
				return
			}
		}
		time.Sleep(10 * time.Second)
	}
}

type Handler interface {
	io.Reader
	io.Writer
	remotecommand.TerminalSizeQueue
}

func ExecInPod(ctx context.Context, client kubernetes.Interface, cfg *rest.Config, h Handler, namespace, name, container string, cmd []string) error {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   cmd,
		Container: container,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		resourceLog.Error(err, "Failed to create SPDY executor")
		return err
	}
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             h,
		Stdout:            h,
		Stderr:            h,
		TerminalSizeQueue: h,
		Tty:               true,
	}); err != nil {
		resourceLog.Error(err, "Failed to stream")
		return err
	}

	return nil
}

func DownloadPodFile(ctx context.Context, client kubernetes.Interface, cfg *rest.Config, writer io.Writer, namespace, name, container string, cmd []string) error {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   cmd,
		Container: container,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		resourceLog.Error(err, "Failed to create SPDY executor")
		return err
	}
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: writer,
		Stderr: writer,
	}); err != nil {
		resourceLog.Error(err, "Failed to stream")
		return err
	}

	return nil
}

func DownloadPodLog(ctx context.Context, client kubernetes.Interface, namespace, name, container, saveFile string) error {
	const safeDir = "/tmp"
	if !strings.HasPrefix(saveFile, safeDir) {
		return fmt.Errorf("invalid file path: %s, must be within %s", saveFile, safeDir)
	}

	// Get the logs
	req := client.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{
		Container: container,
	})

	// Read the logs
	podLogs, err := req.Stream(ctx)
	if err != nil {
		fmt.Println("Error in opening stream:", err)
		return err
	}
	defer podLogs.Close()

	// Open a file to write logs
	file, err := os.OpenFile(saveFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return err
	}
	defer file.Close()

	// Create a buffered writer
	writer := bufio.NewWriter(file)

	// Write the logs to the buffered writer
	_, err = io.Copy(writer, podLogs)
	if err != nil {
		fmt.Println("Error in copying information from podLogs to file:", err)
		return err
	}

	// Flush the buffered writer to ensure all data is written to the file
	err = writer.Flush()
	if err != nil {
		fmt.Println("Error flushing writer:", err)
		return err
	}
	return nil
}
