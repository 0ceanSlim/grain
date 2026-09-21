package config

import (
	"testing"

	cfgType "github.com/0ceanslim/grain/config/types"
)

// database.fulltext_kinds: nil is "not set" (nostrdb applies its default),
// an explicit empty list is a deliberate opt-out, and out-of-range kinds
// are a hard error rather than a warning.
func TestValidate_FulltextKinds(t *testing.T) {
	cases := []struct {
		name    string
		kinds   []int
		wantErr bool
	}{
		{"unset", nil, false},
		{"explicit_empty", []int{}, false},
		{"default_set", []int{0, 1, 30023}, false},
		{"custom", []int{1, 30023, 30818, 9}, false},
		{"negative", []int{1, -1}, true},
		{"too_large", []int{1, 65536}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &cfgType.ServerConfig{}
			cfg.Database.FulltextKinds = tc.kinds
			_, err := ValidateAndApplyDefaults(cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			// nil must survive validation as nil so Open can tell it
			// apart from an explicit []
			if tc.kinds == nil && cfg.Database.FulltextKinds != nil {
				t.Fatalf("nil fulltext_kinds was replaced with %v", cfg.Database.FulltextKinds)
			}
			if tc.kinds != nil && cfg.Database.FulltextKinds == nil {
				t.Fatalf("explicit fulltext_kinds %v was dropped", tc.kinds)
			}
		})
	}
}
