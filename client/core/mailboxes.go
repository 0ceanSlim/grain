package core

// Mailboxes represents a user's relay preferences from NIP-65
type Mailboxes struct {
	Read  []string `json:"read"`
	Write []string `json:"write"`
	Both  []string `json:"both"`
}

// ToStringSlice combines Read, Write, and Both into a single []string
func (m Mailboxes) ToStringSlice() []string {
	var urls []string
	urls = append(urls, m.Read...)
	urls = append(urls, m.Write...)
	urls = append(urls, m.Both...)
	return urls
}
