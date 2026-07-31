package service

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// cryptoUniformInt returns a uniformly distributed integer in [0, n) using
// rejection sampling so modulo bias cannot skew reel stops. The reader is
// injectable for deterministic tests. Seeds are never returned or persisted.
func cryptoUniformInt(r io.Reader, n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("n must be positive")
	}
	if n == 1 {
		return 0, nil
	}
	if r == nil {
		r = rand.Reader
	}

	// Use 32-bit draws; n for reel strips is far below 2^32.
	max := uint32(math.MaxUint32)
	limit := max - (max % uint32(n))
	var buf [4]byte
	for {
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, fmt.Errorf("crypto rand read: %w", err)
		}
		v := binary.BigEndian.Uint32(buf[:])
		if v >= limit {
			continue // reject biased tail
		}
		return int(v % uint32(n)), nil
	}
}

func defaultCryptoReader() io.Reader {
	return rand.Reader
}

// fixedRandReader yields a repeating stream of big-endian uint32 values.
// Intended only for unit tests that need deterministic reel stops.
type fixedRandReader struct {
	values []uint32
	index  int
}

func newFixedRandReader(values ...uint32) *fixedRandReader {
	cp := make([]uint32, len(values))
	copy(cp, values)
	return &fixedRandReader{values: cp}
}

func (r *fixedRandReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(r.values) == 0 {
		return 0, fmt.Errorf("fixed rand reader exhausted")
	}
	n := 0
	for n+4 <= len(p) {
		if r.index >= len(r.values) {
			r.index = 0
		}
		binary.BigEndian.PutUint32(p[n:n+4], r.values[r.index])
		r.index++
		n += 4
	}
	if n == 0 && len(p) > 0 {
		// Partial trailing read: still consume one value.
		if r.index >= len(r.values) {
			r.index = 0
		}
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], r.values[r.index])
		r.index++
		return copy(p, buf[:]), nil
	}
	return n, nil
}
