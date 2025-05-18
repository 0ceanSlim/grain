package eventStore

import (
	"log/slog"
	"strings"

	"github.com/0ceanslim/grain/server/utils"
)

var log *slog.Logger

func init() {
	log = utils.GetLogger("mongo-event")
}

func parseAddressableEventReference(tagA string) []string {
	return strings.Split(tagA, ":")
}