package eventStore

import (
	"strings"
)

func parseAddressableEventReference(tagA string) []string {
	return strings.Split(tagA, ":")
}