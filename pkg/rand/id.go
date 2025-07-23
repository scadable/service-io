package rand

import (
	cr "crypto/rand"
	"encoding/base32"
)

func ID16() string {
	var b [10]byte // 10 raw bytes → 16 base32 chars
	_, _ = cr.Read(b[:])
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}
