package testfixture

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// LegacySource builds an encrypted fixture with the distributed agy format key.
func LegacySource(t testing.TB, path string, raw []byte) {
	t.Helper()
	m := Message(t, "gemini_coder.Trajectory", raw)
	plain, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	LegacyBytes(t, path, plain)
}

func LegacyBytes(t testing.TB, path string, plain []byte) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	key, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../completeness/legacy_key.bin"))
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	data := aead.Seal(nonce, nonce, plain, nil)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
