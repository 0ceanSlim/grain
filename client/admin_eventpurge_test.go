package client

import (
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	cfgType "github.com/0ceanslim/grain/config/types"
)

// The Event Purge form carries the retention clock and late-arrival
// threshold, preselected from config, so saving the section round-trips them.
func TestAdminEventPurgeSection_RetentionClockFields(t *testing.T) {
	// main embeds www/ from the repo root; the test reads the same files.
	SetEmbeddedWWW(os.DirFS(".."))

	for _, tc := range []struct {
		clock, selected string
		late            int
	}{
		{cfgType.RetentionClockReceived, `value="received" selected`, 10},
		{cfgType.RetentionClockCreatedAt, `value="created_at" selected`, 45},
	} {
		rec := httptest.NewRecorder()
		renderAdmin(rec, AdminPageData{
			Title: "admin",
			Owner: strings.Repeat("ab", 32),
			Sections: []AdminSection{{
				ID: "event_purge", Title: "Event purge", Icon: "🧹", Method: "grain_updateeventpurge",
				Config: EventPurgeSectionData{
					Config: cfgType.EventPurgeConfig{
						RetentionClock:     tc.clock,
						LateArrivalMinutes: tc.late,
					},
					Categories: purgeCategoriesFor(nil),
					KindLabels: KindLabels,
				},
			}},
			KindLabels: KindLabels,
		})
		body := rec.Body.String()
		if rec.Code != 200 || strings.Contains(body, "Error executing template") {
			t.Fatalf("clock %q: render failed (%d): %.300s", tc.clock, rec.Code, body)
		}
		if !strings.Contains(body, `name="retention_clock"`) || !strings.Contains(body, tc.selected) {
			t.Errorf("clock %q: retention_clock select missing or not preselected", tc.clock)
		}
		late := regexp.MustCompile(`name="late_arrival_minutes"[\s\S]*?value="(\d+)"`).FindStringSubmatch(body)
		if late == nil || late[1] != strconv.Itoa(tc.late) {
			t.Errorf("clock %q: late_arrival_minutes input = %v, want value %d", tc.clock, late, tc.late)
		}
	}
}
