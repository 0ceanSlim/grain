package utils

// IsHexPubkey reports whether s is a pubkey in its only wire form: 64
// lowercase hex characters. Pubkeys read from outside (mute-list p tags,
// nostr.json names) are checked with it before they're stored or served.
func IsHexPubkey(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
