package log

import "log/slog"

// All logging components defined in one place
func Startup() *slog.Logger          { return GetLogger("startup") }
func DB() *slog.Logger               { return GetLogger("db") }
func RelayClient() *slog.Logger      { return GetLogger("relay-client") }
func RelayConnection() *slog.Logger  { return GetLogger("relay-connection") }
func RelayAPI() *slog.Logger         { return GetLogger("relay-api") }
func Log() *slog.Logger              { return GetLogger("log") }
func Config() *slog.Logger           { return GetLogger("config") }
func Util() *slog.Logger             { return GetLogger("util") }
func Validation() *slog.Logger       { return GetLogger("event-validation") }
func DBQuery() *slog.Logger          { return GetLogger("db-query") }
func DBStore() *slog.Logger          { return GetLogger("db-store") }
func DBPurge() *slog.Logger          { return GetLogger("db-purge") }
func EventStore() *slog.Logger       { return GetLogger("event-store") }
func Event() *slog.Logger            { return GetLogger("event-handler") }
func Req() *slog.Logger              { return GetLogger("req-handler") }
func Auth() *slog.Logger             { return GetLogger("auth-handler") }
func Close() *slog.Logger            { return GetLogger("close-handler") }
func ClientMain() *slog.Logger       { return GetLogger("client-main") }
func ClientAPI() *slog.Logger        { return GetLogger("client-api") }
func ClientCore() *slog.Logger       { return GetLogger("client-core") }
func ClientTools() *slog.Logger      { return GetLogger("client-tools") }
func ClientData() *slog.Logger       { return GetLogger("client-data") }
func ClientConnection() *slog.Logger { return GetLogger("client-connection") }
func ClientSession() *slog.Logger    { return GetLogger("client-session") }
func ClientCache() *slog.Logger      { return GetLogger("client-cache") }

// GetAllComponents returns a slice of all component names used by the logger functions
func GetAllComponents() []string {
	return []string{
		"startup",           // Startup()
		"db",                // DB()
		"relay-client",      // RelayClient()
		"relay-connection",  // RelayConnection()
		"relay-api",         // RelayAPI()
		"log",               // Log()
		"config",            // Config()
		"util",              // Util()
		"event-validation",  // Validation()
		"db-query",          // DBQuery()
		"db-store",          // DBStore()
		"db-purge",          // DBPurge()
		"event-store",       // EventStore()
		"event-handler",     // Event()
		"req-handler",       // Req()
		"auth-handler",      // Auth()
		"close-handler",     // Close()
		"client-main",       // ClientMain()
		"client-api",        // ClientAPI()
		"client-core",       // ClientCore()
		"client-tools",      // ClientTools()
		"client-data",       // ClientData()
		"client-connection", // ClientConnection()
		"client-session",    // ClientSession()
		"client-cache",      // ClientCache()
	}
}
