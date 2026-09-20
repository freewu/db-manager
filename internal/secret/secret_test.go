package secret

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testCipher(t *testing.T) *Cipher {
	t.Helper()
	return Open(t.TempDir())
}

func TestSealUnsealRoundTrip(t *testing.T) {
	c := testCipher(t)

	token, err := c.Seal("s3cret")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if token == "s3cret" || !IsSealed(token) {
		t.Fatalf("the token must not be the password: %q", token)
	}
	if strings.Contains(token, "s3cret") {
		t.Fatalf("the password leaked into the token: %q", token)
	}

	plain, err := c.Unseal(token)
	if err != nil {
		t.Fatalf("unseal: %v", err)
	}
	if plain != "s3cret" {
		t.Fatalf("round trip changed the value: %q", plain)
	}

	// An empty value stays empty: callers do not have to special-case "no
	// password".
	empty, err := c.Seal("")
	if err != nil || empty != "" {
		t.Fatalf("sealing nothing should be a no-op, got %q err=%v", empty, err)
	}
	if plain, err := c.Unseal(""); err != nil || plain != "" {
		t.Fatalf("unsealing nothing should be a no-op, got %q err=%v", plain, err)
	}
}

func TestUnsealPassesPlainTextThrough(t *testing.T) {
	c := testCipher(t)

	// A password an older build wrote is not a token and must survive as-is,
	// otherwise upgrading would silently forget every stored password.
	plain, err := c.Unseal("written-by-an-older-build")
	if err != nil {
		t.Fatalf("unseal plain: %v", err)
	}
	if plain != "written-by-an-older-build" {
		t.Fatalf("plain values must pass through, got %q", plain)
	}
}

func TestUnsealRejectsTamperedTokens(t *testing.T) {
	c := testCipher(t)
	token, err := c.Seal("s3cret")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	body := strings.TrimPrefix(token, TokenPrefix)
	// Flip the last character: GCM authenticates, so this is not a decryption
	// of something else but a value that must be refused.
	last := body[len(body)-1]
	flipped := "A"
	if last == 'A' {
		flipped = "B"
	}
	bad := []string{
		TokenPrefix + body[:len(body)-1] + flipped,
		TokenPrefix + "not base64!",
		TokenPrefix,
		TokenPrefix + "AAAA",
	}
	for _, value := range bad {
		if got, err := c.Unseal(value); err == nil {
			t.Fatalf("Unseal(%q) = %q, want an error", value, got)
		}
	}
}

func TestTokensDoNotOpenWithAnotherKey(t *testing.T) {
	dir := t.TempDir()
	writer := Open(dir)
	token, err := writer.Seal("s3cret")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	// A second machine: same profiles, different key.
	other := Open(t.TempDir())
	if plain, err := other.Unseal(token); err == nil {
		t.Fatalf("a foreign key opened the token: %q", plain)
	}

	// And the key file is what makes the difference: replace it with a fresh
	// one and the first cipher can no longer read its own token either.
	if err := os.Remove(filepath.Join(dir, keyName)); err != nil {
		t.Fatalf("remove key: %v", err)
	}
	if plain, err := Open(dir).Unseal(token); err == nil {
		t.Fatalf("a replaced key opened the token: %q", plain)
	}
}

func TestKeyFileIsCreatedOnceAndKeptPrivate(t *testing.T) {
	dir := t.TempDir()
	c := Open(dir)

	// Nothing is written until a secret actually has to be sealed: an install
	// that never saves a password never gets a key.
	if _, err := os.Stat(filepath.Join(dir, keyName)); !os.IsNotExist(err) {
		t.Fatalf("the key must not be created eagerly (err=%v)", err)
	}
	token, err := c.Seal("s3cret")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, keyName))
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	// Windows does not carry mode bits, so this only means something elsewhere.
	if runtime.GOOS != "windows" && info.Mode().Perm() != keyMode {
		t.Fatalf("key file is %v, want %v", info.Mode().Perm(), os.FileMode(keyMode))
	}

	// A second cipher over the same directory reuses the key instead of making
	// a new one, or every earlier token would become unreadable.
	again := Open(dir)
	if plain, err := again.Unseal(token); err != nil || plain != "s3cret" {
		t.Fatalf("a second cipher over the same directory must reuse the key, got %q err=%v", plain, err)
	}
}

func TestEmptyKeyFileIsReplaced(t *testing.T) {
	dir := t.TempDir()
	// A crash between create and write leaves an empty file; it cannot have
	// sealed anything, so it is regenerated rather than reported as corrupt.
	if err := os.WriteFile(filepath.Join(dir, keyName), nil, keyMode); err != nil {
		t.Fatalf("seed empty key: %v", err)
	}
	c := Open(dir)
	token, err := c.Seal("s3cret")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if plain, err := c.Unseal(token); err != nil || plain != "s3cret" {
		t.Fatalf("round trip after an empty key file: %q err=%v", plain, err)
	}
}

func TestCorruptKeyFileIsReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, keyName), []byte("too-short\n"), keyMode); err != nil {
		t.Fatalf("seed corrupt key: %v", err)
	}
	if _, err := Open(dir).Seal("s3cret"); err == nil {
		t.Fatal("a truncated key must be reported, not silently replaced")
	}
}
