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
	"bytes"
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

type capturedFrame struct {
	payloadType byte
	data        []byte
}

var captureFrameCodec = websocket.Codec{
	Unmarshal: func(data []byte, payloadType byte, v interface{}) error {
		frame := v.(*capturedFrame)
		frame.payloadType = payloadType
		frame.data = append([]byte(nil), data...)
		return nil
	},
}

type hijackResponseWriter struct {
	conn net.Conn
	rw   *bufio.ReadWriter
}

func (w *hijackResponseWriter) Header() http.Header         { return make(http.Header) }
func (w *hijackResponseWriter) Write(p []byte) (int, error) { return w.rw.Write(p) }
func (w *hijackResponseWriter) WriteHeader(int)             {}
func (w *hijackResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.rw, nil
}

func newTerminalWebsocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()

	clientSide, serverSide := net.Pipe()
	deadline := time.Now().Add(5 * time.Second)
	if err := clientSide.SetDeadline(deadline); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	if err := serverSide.SetDeadline(deadline); err != nil {
		t.Fatalf("set server deadline: %v", err)
	}
	serverConnCh := make(chan *websocket.Conn, 1)
	serverErrCh := make(chan error, 1)
	stop := make(chan struct{})
	go func() {
		reader := bufio.NewReader(serverSide)
		req, err := http.ReadRequest(reader)
		if err != nil {
			serverErrCh <- err
			return
		}
		writer := &hijackResponseWriter{
			conn: serverSide,
			rw:   bufio.NewReadWriter(reader, bufio.NewWriter(serverSide)),
		}
		websocket.Handler(func(conn *websocket.Conn) {
			serverConnCh <- conn
			<-stop
		}).ServeHTTP(writer, req)
	}()

	config, err := websocket.NewConfig("ws://example.test/", "http://example.test/")
	if err != nil {
		t.Fatalf("create websocket config: %v", err)
	}
	client, err := websocket.NewClient(config, clientSide)
	if err != nil {
		t.Fatalf("create websocket client: %v", err)
	}
	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConnCh:
	case err := <-serverErrCh:
		t.Fatalf("create websocket server: %v", err)
	}

	t.Cleanup(func() {
		close(stop)
		_ = client.Close()
		_ = serverConn.Close()
	})
	return serverConn, client
}

func newTestTerminalSession(t *testing.T, conn *websocket.Conn) *terminalSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return NewTerminalSession(ctx, conn, EndOfTransmission)
}

func sendFrame(conn *websocket.Conn, payload interface{}) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- websocket.Message.Send(conn, payload)
	}()
	return errCh
}

func TestTerminalSessionReadBinaryFrame(t *testing.T) {
	serverConn, client := newTerminalWebsocketPair(t)
	session := newTestTerminalSession(t, serverConn)
	expected := []byte{0x00, 0x01, 0x7f, 0x80, 0xff}

	sendErr := sendFrame(client, expected)
	actual := make([]byte, len(expected))
	n, err := session.Read(actual)
	if err != nil {
		t.Fatalf("read binary frame: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("send binary frame: %v", err)
	}
	if !bytes.Equal(actual[:n], expected) {
		t.Fatalf("binary frame changed: got %v, want %v", actual[:n], expected)
	}
}

func TestTerminalSessionReadKeepsFrameRemainder(t *testing.T) {
	serverConn, client := newTerminalWebsocketPair(t)
	session := newTestTerminalSession(t, serverConn)
	expected := []byte{0x00, 0x01, 0x02, 0x80, 0xfe, 0xff}

	sendErr := sendFrame(client, expected)
	actual := make([]byte, 0, len(expected))
	for len(actual) < len(expected) {
		buf := make([]byte, 2)
		n, err := session.Read(buf)
		if err != nil {
			t.Fatalf("read binary frame: %v", err)
		}
		actual = append(actual, buf[:n]...)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("send binary frame: %v", err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("binary frame changed: got %v, want %v", actual, expected)
	}
}

func TestTerminalSessionReadJSONStdin(t *testing.T) {
	serverConn, client := newTerminalWebsocketPair(t)
	session := newTestTerminalSession(t, serverConn)
	expected := []byte("echo ok\r")

	sendErr := sendFrame(client, `{"type":"stdin","data":"echo ok\r"}`)
	actual := make([]byte, len(expected))
	n, err := session.Read(actual)
	if err != nil {
		t.Fatalf("read stdin message: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("send stdin message: %v", err)
	}
	if !bytes.Equal(actual[:n], expected) {
		t.Fatalf("stdin changed: got %q, want %q", actual[:n], expected)
	}
}

func TestTerminalSessionHeartbeatIsConcurrentSafe(t *testing.T) {
	session := &terminalSession{}
	session.touchHeartbeat()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 1000 {
			session.touchHeartbeat()
		}
	}()
	go func() {
		defer wg.Done()
		for range 1000 {
			_ = session.heartbeatExpired(time.Minute)
		}
	}()
	wg.Wait()
}

func TestTerminalSessionWriteReturnsNoBytesOnError(t *testing.T) {
	serverConn, _ := newTerminalWebsocketPair(t)
	session := newTestTerminalSession(t, serverConn)
	if err := serverConn.SetDeadline(time.Now()); err != nil {
		t.Fatalf("set closed connection deadline: %v", err)
	}
	_ = serverConn.Close()

	n, err := session.Write([]byte("output"))
	if err == nil {
		t.Fatal("write succeeded after peer closed")
	}
	if n != 0 {
		t.Fatalf("write byte count = %d, want 0", n)
	}
}

func TestTerminalSessionWriteBinaryFrame(t *testing.T) {
	serverConn, client := newTerminalWebsocketPair(t)
	session := newTestTerminalSession(t, serverConn)
	expected := []byte{0x00, 0x01, 0x7f, 0x80, 0xff}

	writeErr := make(chan error, 1)
	go func() {
		_, err := session.Write(expected)
		writeErr <- err
	}()
	var frame capturedFrame
	if err := captureFrameCodec.Receive(client, &frame); err != nil {
		t.Fatalf("receive terminal output: %v", err)
	}
	if err := <-writeErr; err != nil {
		t.Fatalf("write terminal output: %v", err)
	}
	if frame.payloadType != websocket.BinaryFrame {
		t.Fatalf("terminal output frame type = %d, want binary", frame.payloadType)
	}
	if !bytes.Equal(frame.data, expected) {
		t.Fatalf("terminal output changed: got %v, want %v", frame.data, expected)
	}
}
