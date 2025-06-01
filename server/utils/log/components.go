package log

import "log/slog"

// All logging components defined in one place
func Main() *slog.Logger       { return GetLogger("main") }
func Mongo() *slog.Logger      { return GetLogger("mongo") }
func MongoQuery() *slog.Logger { return GetLogger("mongo-query") }
func MongoStore() *slog.Logger { return GetLogger("mongo-store") }
func MongoPurge() *slog.Logger { return GetLogger("mongo-purge") }
func EventStore() *slog.Logger { return GetLogger("event-store") }
func Event() *slog.Logger      { return GetLogger("event-handler") }
func Req() *slog.Logger        { return GetLogger("req-handler") }
func Auth() *slog.Logger       { return GetLogger("auth-handler") }
func Close() *slog.Logger      { return GetLogger("close-handler") }
func Client() *slog.Logger     { return GetLogger("client") }
func Config() *slog.Logger     { return GetLogger("config") }
func Util() *slog.Logger       { return GetLogger("util") }
func Validation() *slog.Logger { return GetLogger("event-validation") }
func ConnMgr() *slog.Logger    { return GetLogger("conn-manager") }
func UserSync() *slog.Logger   { return GetLogger("user-sync") }