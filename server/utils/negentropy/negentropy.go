package negentropy

import (
	"fmt"

	"github.com/illuzen/go-negentropy"
)

// CustomStorage implements the negentropy.Storage interface.
type CustomStorage struct {
	items []negentropy.Item
}

// Size returns the number of items in storage.
func (s *CustomStorage) Size() int {
	return len(s.items)
}

// GetItem retrieves the item at a specific index.
func (s *CustomStorage) GetItem(i uint64) (negentropy.Item, error) {
	if int(i) >= len(s.items) {
		return negentropy.Item{}, fmt.Errorf("index out of bounds")
	}
	return s.items[i], nil
}

// Iterate iterates over a range of items and applies a callback function.
func (s *CustomStorage) Iterate(begin, end int, cb func(item negentropy.Item, i int) bool) error {
	for i := begin; i < end; i++ {
		if !cb(s.items[i], i) {
			break
		}
	}
	return nil
}

// FindLowerBound finds the first item in the range [begin, end) greater than or equal to the value.
func (s *CustomStorage) FindLowerBound(begin, end int, value negentropy.Bound) (int, error) {
	for i := begin; i < end; i++ {
		if !s.items[i].LessThan(value.Item) {
			return i, nil
		}
	}
	return end, nil
}

// Fingerprint calculates the fingerprint for a range of items.
func (s *CustomStorage) Fingerprint(begin, end int) (negentropy.Fingerprint, error) {
	// Validate range
	if begin < 0 || end > len(s.items) || begin > end {
		return negentropy.Fingerprint{}, fmt.Errorf("invalid range for fingerprint: begin=%d, end=%d", begin, end)
	}

	// Initialize the fingerprint as a 16-byte array (Buf is [16]byte)
	var fingerprint [negentropy.FingerprintSize]byte

	// Compute the XOR fingerprint across all items in the range
	for i := begin; i < end; i++ {
		itemID := s.items[i].ID
		for j := 0; j < len(fingerprint) && j < len(itemID); j++ {
			fingerprint[j] ^= itemID[j] // XOR operation
		}
	}

	// Return the computed fingerprint
	return negentropy.Fingerprint{Buf: fingerprint}, nil
}
