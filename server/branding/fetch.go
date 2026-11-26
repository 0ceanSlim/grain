package branding

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/0ceanslim/grain/server/utils/safehttp"
)

// FetchTimeout bounds one fetch of an image URL, connection to last byte. It
// runs inside the admin's NIP-86 request, so it stays under the server's
// write timeout (10 s by default) and the admin always gets an answer.
const FetchTimeout = 8 * time.Second

var fetchClient = safehttp.NewClient(FetchTimeout, 2)

// Fetch downloads an image URL once (http or https only, public addresses
// only, at most MaxImageBytes) and checks it.
func Fetch(ctx context.Context, rawURL string) (Image, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Image{}, fmt.Errorf("not an http(s) URL: %q", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Image{}, err
	}
	req.Header.Set("User-Agent", "grain (relay branding image fetch)")
	req.Header.Set("Accept", "image/png,image/webp,image/jpeg,image/gif,image/x-icon;q=0.9,*/*;q=0.1")
	resp, err := fetchClient.Do(req)
	if err != nil {
		return Image{}, fmt.Errorf("fetch %s: %w", u.Redacted(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Image{}, fmt.Errorf("fetch %s: HTTP %d", u.Redacted(), resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return Image{}, fmt.Errorf("fetch %s: %w", u.Redacted(), err)
	}
	return Check(data)
}
