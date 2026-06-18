package core

import (
	"sync/atomic"

	"github.com/0ceanslim/grain/server/utils/log"
)

// Logger is the structured-logging seam for the importable client library. It
// matches the method set of *slog.Logger, so the standard library logger — and
// grain's own log.ClientCore() — satisfy it as-is. A consumer that doesn't want
// to be tied to grain's logging plugs in their own with SetLogger.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// loggerBox keeps the atomic.Value's stored concrete type constant (it panics on
// a type change) while letting the wrapped Logger vary between the default and a
// consumer-supplied one.
type loggerBox struct{ Logger }

// coreLogger holds the active loggerBox. It defaults to grain's client-core slog
// logger, so behaviour is unchanged until a consumer calls SetLogger.
var coreLogger atomic.Value

func init() {
	coreLogger.Store(loggerBox{log.ClientCore()})
}

// SetLogger replaces the process-wide logger the client library writes through.
// It is package-global by design: the library logs from many package-level
// functions with no Client in scope, so threading a per-Client logger everywhere
// would add a parameter to most of the surface for little gain. Call it once at
// startup. A nil logger is ignored; passing log.ClientCore() restores the
// default. Safe for concurrent use.
func SetLogger(l Logger) {
	if l != nil {
		coreLogger.Store(loggerBox{l})
	}
}

// clog returns the active client-library logger.
func clog() Logger {
	return coreLogger.Load().(loggerBox).Logger
}
