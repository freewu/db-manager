package singleinstance

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The two files live in the per-user cache directory, so the tests move that
// directory into a temp one instead of claiming the real application — a claim
// taken here would keep the real app from starting while the tests run. Windows
// reads %LocalAppData%, everything else reads $XDG_CACHE_HOME.
func withCacheHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("LocalAppData", home)
	} else {
		t.Setenv("XDG_CACHE_HOME", home)
	}
	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lock dir: %v", err)
	}
	if !strings.HasPrefix(dir, home) {
		t.Fatalf("lock dir %s is not under the temp home %s", dir, home)
	}
	return dir
}

// A second copy is refused while the first holds the claim, and letting go is
// what makes the next launch a copy again — including taking the published
// address with it, so nothing can reach a window that is no longer there.
func TestSecondCopyIsRefusedUntilTheFirstLetsGo(t *testing.T) {
	dir := withCacheHome(t)
	first, err := Acquire()
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if _, err := Acquire(); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second claim: got %v, want ErrAlreadyRunning", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, recordName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the record is still there after release: %v", err)
	}

	second, err := Acquire()
	if err != nil {
		t.Fatalf("claim after release: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

// A launch that has been refused the claim reaches the copy that holds it, and
// the window is asked for before the launch is told it was.
func TestActivateReachesTheRunningCopy(t *testing.T) {
	withCacheHome(t)
	running, err := Acquire()
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	defer func() { _ = running.Close() }()

	shown := make(chan struct{}, 1)
	running.Serve(func() { shown <- struct{}{} })

	if err := Activate(); err != nil {
		t.Fatalf("activate: %v", err)
	}
	select {
	case <-shown:
	case <-time.After(requestTimeout):
		t.Fatal("the running copy was never asked to show its window")
	}
}

// With nothing running there is nothing to reach: a launch in that state takes
// the claim and starts, so this error is only ever seen by a test.
func TestActivateWithoutARunningCopy(t *testing.T) {
	withCacheHome(t)
	if err := Activate(); err == nil {
		t.Fatal("activate succeeded with nothing running")
	}
}

// The running copy answers its own token only, and a request that is not carrying
// it shows nothing — otherwise anything that found the port could pull the window
// out from under the user.
func TestRequestWithTheWrongTokenIsRefused(t *testing.T) {
	dir := withCacheHome(t)
	running, err := Acquire()
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	defer func() { _ = running.Close() }()

	shown := make(chan struct{}, 1)
	running.Serve(func() { shown <- struct{}{} })

	target, err := readRecord(filepath.Join(dir, recordName))
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	reply := ask(t, target.Port, commandShow+" "+target.Token+"-not")
	if reply != replyRefuse {
		t.Fatalf("a wrong token was answered %q, want %q", reply, replyRefuse)
	}
	select {
	case <-shown:
		t.Fatal("a request that was not ours showed the window")
	default:
	}
}

// A record left behind by a copy that died is not believed: the address it names
// belongs to whatever holds that port now, and an answer that does not come from
// this application is not taken for the running copy.
func TestStaleRecordIsNotMistakenForARunningCopy(t *testing.T) {
	dir := withCacheHome(t)
	stranger, err := net.Listen("tcp", loopback+":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer stranger.Close()
	go func() {
		conn, err := stranger.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.WriteString(conn, "hello\n")
	}()

	port := stranger.Addr().(*net.TCPAddr).Port
	raw, err := json.Marshal(record{Version: schemaVer, Port: port, Token: "someone-elses"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, recordName), raw, fileMode); err != nil {
		t.Fatalf("write record: %v", err)
	}

	if err := Activate(); err == nil {
		t.Fatal("activate took a stranger's answer for the running copy")
	}
}

// ask sends one line to a port and returns the answer, so a test can speak to
// the running copy the way a launch does.
func ask(t *testing.T, port int, line string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(loopback, strconv.Itoa(port)), requestTimeout)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(requestTimeout))
	if _, err := io.WriteString(conn, line+"\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw := make([]byte, 16)
	n, err := conn.Read(raw)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return strings.TrimSpace(string(raw[:n]))
}
