package client

import "io/fs"

var wwwFS fs.FS

// SetEmbeddedWWW sets the embedded filesystem containing www/ static assets.
// The FS should have "www" as the root prefix (e.g., files at "www/views/app.html").
func SetEmbeddedWWW(fsys fs.FS) { wwwFS = fsys }

// GetEmbeddedWWW returns the embedded www filesystem.
func GetEmbeddedWWW() fs.FS { return wwwFS }
