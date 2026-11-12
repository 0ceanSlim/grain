// Package branding keeps grain's own copy of the relay's branding images
// (favicon, logo, light-theme logo, banner) and serves them, so visitors'
// browsers never fetch them from an outside host and they survive the
// original source going away.
//
// An image comes in from the owner (an upload, or a URL grain fetches once),
// is checked (a raster image of an allowed type, at most MaxImageBytes), and
// is written to <data dir>/branding/<slot>-<hash>.<ext>, where the data dir
// is the persistent volume that also holds relay_metadata.json. grain serves
// it at /branding/<file>; the relay metadata field for the slot then holds
// that self-hosted URL. The file name carries a content hash, so the URL
// changes whenever the image does and can be cached forever.
package branding

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // DecodeConfig for the allowed raster types
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MaxImageBytes caps one branding image. 700 KiB keeps an upload, base64
// encoded inside a NIP-86 request, under the 1 MiB cap every authenticated
// admin body already has, so storing an image needs no larger body limit.
const MaxImageBytes = 700 << 10

// MaxSide caps either dimension (where grain can read it), so an image that
// is tiny on disk cannot ask every visitor's browser to decode a huge bitmap.
const MaxSide = 8192

// URLPrefix is where grain serves the stored images.
const URLPrefix = "/branding/"

// Slots are the images grain stores, each named after the relay_metadata.json
// field that points at it.
var Slots = []string{"icon", "logo", "logo_light", "banner"}

// IsSlot reports whether s is a branding slot.
func IsSlot(s string) bool {
	for _, v := range Slots {
		if v == s {
			return true
		}
	}
	return false
}

// types are the accepted image types (as http.DetectContentType names them)
// and the extension each is stored and served under. SVG is refused: it can
// carry script, and served from the relay's own origin it would run with the
// admin dashboard's privileges if opened directly. Sanitizing SVG safely is a
// project of its own; a PNG or WebP export does the same job.
var types = map[string]string{
	"image/png":    ".png",
	"image/jpeg":   ".jpg",
	"image/gif":    ".gif",
	"image/webp":   ".webp",
	"image/x-icon": ".ico",
}

var contentTypes = func() map[string]string {
	m := make(map[string]string, len(types))
	for t, ext := range types {
		m[ext] = t
	}
	return m
}()

// Image is a checked image ready to store.
type Image struct {
	Data          []byte
	Type          string // e.g. image/png
	Width, Height int    // 0 when grain cannot read the dimensions (WebP, ICO)
}

// Check validates raw image bytes: non-empty, at most MaxImageBytes, an
// allowed type by content (never by a claimed name or header), and, for the
// types the standard library reads, a well-formed header within MaxSide.
func Check(data []byte) (Image, error) {
	if len(data) == 0 {
		return Image{}, errors.New("empty image")
	}
	if len(data) > MaxImageBytes {
		return Image{}, fmt.Errorf("image is %d bytes; the limit is %d (700 KiB)", len(data), MaxImageBytes)
	}
	typ := http.DetectContentType(data)
	if _, ok := types[typ]; !ok {
		return Image{}, fmt.Errorf("unsupported image type %q: use PNG, JPEG, WebP, GIF or ICO (SVG is not accepted)", typ)
	}
	img := Image{Data: data, Type: typ}
	switch typ {
	case "image/png", "image/jpeg", "image/gif":
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return Image{}, fmt.Errorf("unreadable %s: %v", typ, err)
		}
		img.Width, img.Height = cfg.Width, cfg.Height
		if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > MaxSide || cfg.Height > MaxSide {
			return Image{}, fmt.Errorf("image is %dx%d; each side must be 1 to %d px", cfg.Width, cfg.Height, MaxSide)
		}
	}
	return img, nil
}

// DecodeDataURL reads an upload sent as a data: URL
// ("data:<type>;base64,<payload>"). The claimed type is ignored; Check
// sniffs the bytes.
func DecodeDataURL(s string) ([]byte, error) {
	rest, ok := strings.CutPrefix(s, "data:")
	if !ok {
		return nil, errors.New("not a data: URL")
	}
	meta, payload, ok := strings.Cut(rest, ",")
	if !ok || !strings.HasSuffix(meta, ";base64") {
		return nil, errors.New("data: URL must be base64")
	}
	if base64.StdEncoding.DecodedLen(len(payload)) > MaxImageBytes+3 {
		return nil, fmt.Errorf("image exceeds %d bytes", MaxImageBytes)
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("bad base64: %v", err)
	}
	return data, nil
}

// dir is where images are stored; set by SetDir at startup (the data dir's
// branding/ subfolder).
var dir string

// SetDir sets the storage folder, e.g. config.ConfigPath("branding").
func SetDir(d string) { dir = d }

// Dir returns the storage folder ("" until SetDir).
func Dir() string { return dir }

// fileRE is the only file name shape grain writes or serves.
var fileRE = regexp.MustCompile(`^(icon|logo|logo_light|banner)-[0-9a-f]{16}\.(png|jpg|gif|webp|ico)$`)

// Store writes img for slot and returns its file name. The write is atomic
// (temp file + rename) and idempotent: the same bytes give the same name.
func Store(slot string, img Image) (string, error) {
	if !IsSlot(slot) {
		return "", fmt.Errorf("unknown branding slot %q", slot)
	}
	if dir == "" {
		return "", errors.New("branding storage not configured")
	}
	sum := sha256.Sum256(img.Data)
	name := slot + "-" + hex.EncodeToString(sum[:8]) + types[img.Type]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-"+slot+"-")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(img.Data)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmpName, 0644)
	}
	if werr == nil {
		werr = os.Rename(tmpName, filepath.Join(dir, name))
	}
	if werr != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write branding image: %w", werr)
	}
	return name, nil
}

// Prune removes slot's earlier images, keeping keep. Only names grain itself
// writes for that slot are touched.
func Prune(slot, keep string) {
	if dir == "" || !IsSlot(slot) {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if n == keep || e.IsDir() || !fileRE.MatchString(n) {
			continue
		}
		if strings.HasPrefix(n, slot+"-") {
			_ = os.Remove(filepath.Join(dir, n))
		}
	}
}

// ServeHTTP serves a stored image at /branding/<file>. Unknown names are 404;
// the type comes from the extension grain chose, never from sniffing, and the
// response may not run anything.
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, URLPrefix)
	if dir == "" || !fileRE.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", contentTypes[filepath.Ext(name)])
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Access-Control-Allow-Origin", "*")
	http.ServeContent(w, r, name, st.ModTime(), f)
}
