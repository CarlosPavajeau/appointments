package crypto

import (
	"strings"
	"testing"
)

func TestNewService(t *testing.T) {
	t.Parallel()

	validKey := make([]byte, 32)
	for i := range validKey {
		validKey[i] = byte(i)
	}

	tests := []struct {
		name    string
		key     []byte
		wantErr bool
	}{
		{name: "nil", key: nil, wantErr: true},
		{name: "empty", key: []byte{}, wantErr: true},
		{name: "31 bytes", key: make([]byte, 31), wantErr: true},
		{name: "33 bytes", key: make([]byte, 33), wantErr: true},
		{name: "32 bytes", key: validKey, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, err := NewService(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if svc != nil {
					t.Fatal("expected nil service on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}
			if svc == nil {
				t.Fatal("expected non-nil service")
			}
		})
	}
}

func TestServiceEncryptDecrypt(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	svc, err := NewService(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "hello, token"
	encoded, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if encoded == "" {
		t.Fatal("expected non-empty ciphertext")
	}

	out, err := svc.Decrypt(encoded)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if out != plaintext {
		t.Fatalf("got %q, want %q", out, plaintext)
	}
}

func TestServiceEncryptUsesRandomNonce(t *testing.T) {
	t.Parallel()

	key := bytes32(42)
	svc, err := NewService(key)
	if err != nil {
		t.Fatal(err)
	}

	a, err := svc.Encrypt("same")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Encrypt("same")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("expected two ciphertexts to differ (random nonce per encrypt)")
	}
}

func TestNewServiceCopiesKey(t *testing.T) {
	t.Parallel()

	key := bytes32(7)
	svc, err := NewService(key)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate caller's slice; service must still decrypt what it encrypted.
	for i := range key {
		key[i] = 0
	}

	enc, err := svc.Encrypt("payload")
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if out != "payload" {
		t.Fatalf("got %q after key mutation", out)
	}
}

func bytes32(seed byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed + byte(i)
	}
	return b
}

func TestServiceDecryptWrongKey(t *testing.T) {
	t.Parallel()

	a, err := NewService(bytes32(1))
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewService(bytes32(2))
	if err != nil {
		t.Fatal(err)
	}

	enc, err := a.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Decrypt(enc)
	if err == nil {
		t.Fatal("expected decrypt failure with different key")
	}
}

func TestServiceDecryptInvalidInput(t *testing.T) {
	t.Parallel()

	svc, err := NewService(bytes32(3))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for garbage input")
	}
}

func TestServiceDecryptTruncatedCiphertext(t *testing.T) {
	t.Parallel()

	svc, err := NewService(bytes32(4))
	if err != nil {
		t.Fatal(err)
	}
	// Valid base64 but too short to hold nonce + tag.
	_, err = svc.Decrypt("YQ==") // "a" decoded — far too short for GCM
	if err == nil {
		t.Fatal("expected error for truncated ciphertext")
	}
}

func FuzzServiceRoundTrip(f *testing.F) {
	key := bytes32(9)
	svc, err := NewService(key)
	if err != nil {
		f.Fatal(err)
	}
	f.Add("hello")
	f.Add("")
	f.Add(strings.Repeat("x", 4096))

	f.Fuzz(func(t *testing.T, plaintext string) {
		enc, err := svc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		out, err := svc.Decrypt(enc)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if out != plaintext {
			t.Fatalf("round-trip: got len %d, want len %d", len(out), len(plaintext))
		}
	})
}
