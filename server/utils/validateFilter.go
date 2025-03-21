package utils

import (
	"regexp"

	relay "github.com/0ceanslim/grain/server/types"
)

// isValidHex validates if the given string is a 64-character lowercase hex string
func isValidHex(str string) bool {
	return len(str) == 64 && regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(str)
}

// ValidateFilters ensures the IDs, Authors, and Tags follow the correct hex format
func ValidateFilters(filters []relay.Filter) bool {
	for _, f := range filters {
		// Validate IDs
		for _, id := range f.IDs {
			if !isValidHex(id) {
				return false
			}
		}
		// Validate Authors
		for _, author := range f.Authors {
			if !isValidHex(author) {
				return false
			}
		}
		// Validate Tags
		for _, tags := range f.Tags {
			for _, tag := range tags {
				if !isValidHex(tag) {
					return false
				}
			}
		}
	}
	return true
}
