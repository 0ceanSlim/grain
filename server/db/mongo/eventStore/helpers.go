package eventStore

import (
	"log/slog"
	"strings"

	"github.com/0ceanslim/grain/server/utils"
)

// Set the logging component for eventStore operations
func esLog() *slog.Logger {
	return utils.GetLogger("mongo-es")
}

func parseAddressableEventReference(tagA string) []string {
	return strings.Split(tagA, ":")
}