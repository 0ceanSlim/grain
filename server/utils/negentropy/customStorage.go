package negentropy

import (
	"fmt"
	"log"

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

func (s *CustomStorage) FindLowerBound(begin, end int, value negentropy.Bound) (int, error) {
	log.Printf("FindLowerBound called: begin=%d, end=%d, value=%+v", begin, end, value)
	for i := begin; i < end; i++ {
		if !s.items[i].LessThan(value.Item) {
			return i, nil
		}
	}
	return end, nil
}

func (s *CustomStorage) Iterate(begin, end int, cb func(item negentropy.Item, i int) bool) error {
	log.Printf("Iterate called: begin=%d, end=%d", begin, end)
	for i := begin; i < end; i++ {
		if !cb(s.items[i], i) {
			log.Printf("Iteration stopped early at index %d", i)
			break
		}
	}
	return nil
}

// Fingerprint calculates the fingerprint for a range of items.
func (s *CustomStorage) Fingerprint(begin, end int) (negentropy.Fingerprint, error) {
	if begin < 0 || end > len(s.items) || begin > end {
		return negentropy.Fingerprint{}, fmt.Errorf("invalid range for fingerprint: begin=%d, end=%d", begin, end)
	}

	var fingerprint [negentropy.FingerprintSize]byte
	for i := begin; i < end; i++ {
		itemID := s.items[i].ID
		for j := 0; j < len(fingerprint) && j < len(itemID); j++ {
			fingerprint[j] ^= itemID[j]
		}
	}

	return negentropy.Fingerprint{Buf: fingerprint}, nil
}

func (s *CustomStorage) ValidateIDs() error {
	for _, item := range s.items {
		if len(item.ID) != 32 { // 32 bytes for raw binary
			return fmt.Errorf("invalid ID length: expected 32, got %d (ID: %x)", len(item.ID), item.ID)
		}
	}
	return nil
}
