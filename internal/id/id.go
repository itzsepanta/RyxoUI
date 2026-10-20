// Package id generates sortable, collision-resistant identifiers.
//
// Identifiers are 26-character Crockford base32 strings (ULID layout): a
// 48-bit millisecond timestamp followed by 80 bits of randomness. They are
// sortable by creation time, which makes them usable as pagination cursors,
// and they carry no machine or user information.
package id

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// crockford is the base32 alphabet used by the ULID spec.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Length is the number of characters in a generated identifier.
const Length = 26

var (
	mu       sync.Mutex
	lastMS   int64
	lastRand [10]byte
)

// New returns a new identifier.
func New() string {
	now := time.Now().UTC().UnixMilli()

	mu.Lock()
	defer mu.Unlock()

	if now == lastMS {
		// Same millisecond: increment the random component so identifiers
		// stay strictly increasing instead of relying on randomness alone.
		increment(&lastRand)
	} else {
		lastMS = now
		if _, err := rand.Read(lastRand[:]); err != nil {
			panic(fmt.Sprintf("id: entropy failure: %v", err))
		}
	}

	var buf [16]byte
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(now))
	copy(buf[0:6], ts[2:8])
	copy(buf[6:], lastRand[:])

	return encode(buf)
}

// Timestamp returns the creation time encoded in id.
func Timestamp(id string) (time.Time, error) {
	buf, err := decode(id)
	if err != nil {
		return time.Time{}, err
	}
	ms := int64(buf[0])<<40 | int64(buf[1])<<32 | int64(buf[2])<<24 |
		int64(buf[3])<<16 | int64(buf[4])<<8 | int64(buf[5])
	return time.UnixMilli(ms).UTC(), nil
}

// increment adds one to the big-endian random component.
func increment(b *[10]byte) {
	for i := len(b) - 1; i >= 0; i-- {
		b[i]++
		if b[i] != 0 {
			return
		}
	}
}

func encode(buf [16]byte) string {
	// 128 bits rendered as 26 base32 characters (130 bits, zero padded).
	var bits [26]byte
	// Treat the 16 bytes as a big-endian bit stream.
	for i := 0; i < 16; i++ {
		for j := 0; j < 8; j++ {
			bit := (buf[i] >> (7 - j)) & 1
			pos := i*8 + j
			if bit == 1 {
				bits[pos/5] |= 1 << (4 - pos%5)
			}
		}
	}
	out := make([]byte, Length)
	for i, v := range bits {
		out[i] = crockford[v]
	}
	return string(out)
}

func decode(s string) ([16]byte, error) {
	var out [16]byte
	if len(s) != Length {
		return out, fmt.Errorf("id: invalid length %d", len(s))
	}
	var bits [26]byte
	for i := 0; i < Length; i++ {
		idx := indexOf(s[i])
		if idx < 0 {
			return out, fmt.Errorf("id: invalid character %q", s[i])
		}
		bits[i] = byte(idx)
	}
	for i := 0; i < 16; i++ {
		for j := 0; j < 8; j++ {
			pos := i*8 + j
			if bits[pos/5]&(1<<(4-pos%5)) != 0 {
				out[i] |= 1 << (7 - j)
			}
		}
	}
	return out, nil
}

func indexOf(c byte) int {
	for i := 0; i < len(crockford); i++ {
		if crockford[i] == c {
			return i
		}
	}
	switch c {
	case 'I', 'L', 'O', 'U':
		return -1 // excluded by the alphabet
	}
	// Lower case is accepted for convenience.
	for i := 0; i < len(crockford); i++ {
		if crockford[i] == c+('a'-'A') {
			return i
		}
	}
	return -1
}
