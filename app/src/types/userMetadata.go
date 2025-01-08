package types

type UserMetadata struct {
	DisplayName string `json:"display_name"`
	Picture     string `json:"picture"`
	About       string `json:"about"`
	// can add more extra metadata fields if desired website, banner, name
}
