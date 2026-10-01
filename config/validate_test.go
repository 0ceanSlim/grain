package config

import (
	"testing"

	cfgType "github.com/0ceanslim/grain/config/types"
)

func TestValidate_EventPurgeClock(t *testing.T) {
	cfg := &cfgType.ServerConfig{}
	if _, err := ValidateAndApplyDefaults(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.EventPurge.RetentionClock != cfgType.RetentionClockReceived {
		t.Errorf("retention_clock defaulted to %q, want %q", cfg.EventPurge.RetentionClock, cfgType.RetentionClockReceived)
	}
	if cfg.EventPurge.LateArrivalMinutes != cfgType.DefaultLateArrivalMinutes {
		t.Errorf("late_arrival_minutes defaulted to %d", cfg.EventPurge.LateArrivalMinutes)
	}

	for _, tc := range []struct {
		clock   string
		late    int
		wantErr bool
	}{
		{cfgType.RetentionClockReceived, 30, false},
		{cfgType.RetentionClockCreatedAt, 0, false},
		{"recieved", 0, true},
		{cfgType.RetentionClockReceived, -1, true},
	} {
		cfg := &cfgType.ServerConfig{}
		cfg.EventPurge.RetentionClock = tc.clock
		cfg.EventPurge.LateArrivalMinutes = tc.late
		if _, err := ValidateAndApplyDefaults(cfg); (err != nil) != tc.wantErr {
			t.Errorf("clock %q late %d: err = %v, wantErr %v", tc.clock, tc.late, err, tc.wantErr)
		}
	}
}

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
