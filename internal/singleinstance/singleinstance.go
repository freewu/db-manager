// Package singleinstance keeps the application to one running copy.
//
// A second copy must not start: two of them would have the same profiles, the
// same change log and the same state file open, and would take turns
// overwriting each other's writes. So a launch that is refused the claim does
// not start — it asks the copy that is already running to bring its window to
// the front, and leaves. Bringing the window forward is what keeps that from
// looking like nothing happened, which is how it would look: the copy that is
// running is usually hidden behind the notification-area icon.
//
// The claim is an OS lock on a file rather than a flag inside one, so it is the
// operating system that answers "is a copy running?", and a copy that is killed
// — which is how a desktop application usually goes away — releases it as part
// of dying. The file the lock is on stays behind, empty: what it *contains* is
// never the claim, and a stale one cannot keep the app from starting.
//
// Both files live in the per-user cache directory, not in the data directory,
// for a reason worth writing down: the data directory can be moved by the
// settings page, and a file this process holds open cannot travel — on Windows
// the move would fail outright, and everywhere else the claim would be left on a
// file the app no longer uses. A cache directory is where per-user run-time
// state belongs, nothing in the app reads it, and nothing in it is ever reported
// to the user as data.
package singleinstance

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrAlreadyRunning is what Acquire answers when another copy holds the claim.
// It is the one answer a caller has to tell apart: every other error means the
// claim could not be taken at all.
var ErrAlreadyRunning = errors.New("another copy of the application is running")

const (
	// dirName is the folder under the cache directory the two files live in.
	dirName = "db-manager"
	// lockName is the file whose OS lock *is* the claim. Nothing is ever written
	// to it: the lock is held, not stored, and a file left over by a crash is
	// just an empty file.
	lockName = "instance.lock"
	// recordName is where the copy holding the claim writes how to reach it. Only
	// a launch that has just been refused the claim reads it.
	recordName = "instance.json"
	// schemaVer is the version of the record above. A record that does not carry
	// the version this build writes is not read: it was left by some other build,
	// and the lock — not the record — is what says whether a copy is running.
	schemaVer = 1
	// requestTimeout bounds every step between the two copies. A window that
	// takes longer than this to be asked is not worth keeping the departing
	// launch alive for.
	requestTimeout = 3 * time.Second

	// fileMode and dirMode match the rest of the app's files.
	fileMode = 0o600
	dirMode  = 0o700
)

// loopback is the only address the running copy listens on. It is not reachable
// from outside the machine, which is the whole point: this is a door for the
// next launch, not a service.
const loopback = "127.0.0.1"

// The two lines the copies say to each other: a request carries the command and
// the token from the record, and the answer is one word.
const (
	commandShow = "show"
	replyOK     = "ok"
	replyRefuse = "no"
)

// record is what the running copy leaves for the next launch: the loopback port
// it listens on, and a token so that an answer can be told apart from a
// stranger's.
//
// The token is not a secret — whoever can read this file (it is written 0600,
// so the same user) can use it. It is there so that a *mistake* cannot look like
// a success: a record left by a copy that died, whose port has since been handed
// to something else, would otherwise be answered by a process that has never
// heard of this application.
type record struct {
	Version int    `json:"version"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
}

// Lock is one process's claim that it is the copy that runs, and the address
// later launches knock on.
//
// It comes from Acquire and is held until Close. The app holds exactly one, and
// it holds it from before the store is opened until the pools are shut down.
type Lock struct {
	dir      string
	file     *os.File
	listener net.Listener
	token    string
}

// Acquire takes the claim on running.
//
// It answers ErrAlreadyRunning when another copy holds it; the caller then owns
// nothing and must leave.
//
// The listener is opened before the record is written, so the address the next
// launch is given is one that answers. If it cannot be opened at all the claim is
// still taken: a copy that cannot be asked to come to the front is a worse second
// launch, whereas two copies running is the thing this package exists to prevent.
func Acquire() (*Lock, error) {
	dir, err := lockDir()
	if err != nil {
		return nil, err
	}
	file, err := lockFile(filepath.Join(dir, lockName))
	if err != nil {
		return nil, err
	}

	lock := &Lock{dir: dir, file: file, token: newToken()}
	if listener, err := net.Listen("tcp", loopback+":0"); err == nil {
		lock.listener = listener
		// Best effort: a record that cannot be written costs a later launch its
		// window and nothing else.
		_ = lock.writeRecord()
	}
	return lock, nil
}

// Serve answers later launches: every request that carries this copy's token
// calls show.
//
// It is meant to be called once there is a window to show, which is why it is
// not part of Acquire: the claim is taken before Wails has a window, and it is
// the window, not the claim, that makes answering possible. A request that
// arrives in between is not lost — the connection is already in the listen
// queue, and it is read as soon as the loop starts.
func (l *Lock) Serve(show func()) {
	if l.listener == nil || show == nil {
		return
	}
	go func() {
		for {
			conn, err := l.listener.Accept()
			if err != nil {
				// Only Close ends this loop, and Close means the claim is going
				// away — there is no window left to bring forward.
				return
			}
			go l.serveRequest(conn, show)
		}
	}()
}

// Close gives up the claim, which is what lets the next launch start a copy.
//
// The record goes first and the lock second, and the order matters: while the
// lock is held no other copy can be running, so removing the record cannot
// delete an address that a copy which has just taken over is relying on.
func (l *Lock) Close() error {
	if l.listener != nil {
		_ = l.listener.Close()
	}
	// Best effort: a record that cannot be removed is the same stale hint a crash
	// would have left, and it costs the next launch its window.
	_ = os.Remove(filepath.Join(l.dir, recordName))
	return l.file.Close()
}

// Activate asks the copy that holds the claim to bring its window to the front.
//
// It is called by a launch that has just been refused the claim, so it does not
// answer "is anything running?" — the claim already answered that. All it does is
// turn a launch that cannot happen into one whose effect the user can see, and
// every way it can fail (no record, a record that has gone stale, a copy on its
// way out) ends the same way: the caller leaves, which is what one copy at a time
// means.
func Activate() error {
	dir, err := lockDir()
	if err != nil {
		return err
	}
	target, err := readRecord(filepath.Join(dir, recordName))
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(loopback, strconv.Itoa(target.Port)), requestTimeout)
	if err != nil {
		return fmt.Errorf("reach the running copy: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(requestTimeout))

	if _, err := fmt.Fprintf(conn, "%s %s\n", commandShow, target.Token); err != nil {
		return fmt.Errorf("ask the running copy to come to the front: %w", err)
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("wait for the running copy: %w", err)
	}
	if strings.TrimSpace(reply) != replyOK {
		return errors.New("the running copy did not accept the request")
	}
	return nil
}

// serveRequest answers one launch. The token is compared as plain text on
// purpose: it guards against a stranger's reply, not against a local process,
// which could read the record anyway.
func (l *Lock) serveRequest(conn net.Conn, show func()) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(requestTimeout))

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}
	command, token, ok := strings.Cut(strings.TrimSpace(line), " ")
	if !ok || command != commandShow || l.token == "" || token != l.token {
		_, _ = io.WriteString(conn, replyRefuse+"\n")
		return
	}
	// The window is asked for before the answer goes out, so a launch that is
	// told "ok" is told about something that has already happened.
	show()
	_, _ = io.WriteString(conn, replyOK+"\n")
}

// writeRecord publishes the address this copy can be reached on. It is written
// in place rather than through a temporary file: its only reader is a launch
// that has just been refused the claim, and half an address is a request that
// goes nowhere, never a wrong answer about who is running. The claim is the
// lock; this file is only a hint about how to knock.
func (l *Lock) writeRecord() error {
	addr, ok := l.listener.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("listener address %s is not a TCP address", l.listener.Addr())
	}
	raw, err := json.Marshal(record{Version: schemaVer, Port: addr.Port, Token: l.token})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.dir, recordName), raw, fileMode)
}

// readRecord reads the address the running copy published. A record that is
// missing, unreadable, half-written or written by another build is an error:
// none of them is a reason to refuse to leave, only a reason why the window did
// not come forward.
func readRecord(path string) (record, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return record{}, fmt.Errorf("reach the running copy: %w", err)
	}
	var parsed record
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return record{}, fmt.Errorf("reach the running copy: %w", err)
	}
	if parsed.Version != schemaVer || parsed.Port <= 0 || parsed.Token == "" {
		return record{}, errors.New("the running copy left no usable address")
	}
	return parsed, nil
}

// newToken is what the running copy answers to. A failure to read random bytes
// leaves the token empty, and an empty token answers nothing: the request then
// goes unanswered, which is what a launch that cannot be served already is.
func newToken() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(raw[:])
}

// lockDir is the folder the claim and the record live in: the per-user cache
// directory, which is where run-time state belongs.
func lockDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		// A machine with no cache directory is unusual enough that the temporary
		// directory is a better answer than failing to start. It is per-user
		// where it matters (Windows), which is the property the claim needs.
		base = os.TempDir()
	}
	dir := filepath.Join(base, dirName)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return dir, nil
}
