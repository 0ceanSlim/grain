package api

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"github.com/0ceanslim/grain/server/branding"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// brandingMu serializes every write to a branding image field with the
// prune after it, so a prune never removes a file a concurrent store has
// written but not yet pointed its field at.
var brandingMu sync.Mutex

// pruneBrandingImages removes the stored images no branding field points
// at any more (replaced, cleared, or switched to an outside URL). Call with
// brandingMu held.
func pruneBrandingImages() {
	m := utils.GetRelayMetadataCopy()
	branding.Prune(m.Icon, m.Logo, m.LogoLight, m.Banner)
}

// runStoreBrandingImage is grain_storebrandingimage: params
// [slot, source], where slot is icon / logo / logo_light / banner and
// source is either an upload as a data: URL or an http(s) URL grain fetches
// once. grain keeps its own copy (see package branding), points the slot's
// relay_metadata.json field at it (/branding/<file>), and returns {url,
// type, width, height, bytes}. Owner-only like every NIP-86 method. To clear
// a slot, set its field to "" with changerelayicon / changerelaybanner /
// grain_changerelaylogo / grain_changerelaylogolight.
func runStoreBrandingImage(params []any, signer string) (any, string) {
	slot, ok := paramString(params, 0)
	if !ok || !branding.IsSlot(slot) {
		return nil, "invalid slot: want one of " + strings.Join(branding.Slots, ", ")
	}
	source, ok := paramString(params, 1)
	source = strings.TrimSpace(source)
	if !ok || source == "" {
		return nil, "missing image"
	}
	return storeBrandingSource(slot, source, signer)
}

// storeBrandingSource checks and stores one image (a data: upload or an
// http(s) URL fetched once) for slot and repoints the slot's field at it.
func storeBrandingSource(slot, source, signer string) (map[string]any, string) {
	var (
		img  branding.Image
		err  error
		from string
	)
	switch {
	case strings.HasPrefix(source, "data:"):
		from = "upload"
		var data []byte
		if data, err = branding.DecodeDataURL(source); err == nil {
			img, err = branding.Check(data)
		}
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		from = source
		if u, perr := url.Parse(source); perr == nil {
			u.User = nil // keep credentials out of the log
			from = u.String()
		}
		ctx, cancel := context.WithTimeout(context.Background(), branding.FetchTimeout)
		img, err = branding.Fetch(ctx, source)
		cancel()
	default:
		return nil, "image must be an upload (data: URL) or an http(s) URL"
	}
	if err != nil {
		return nil, err.Error()
	}
	brandingMu.Lock()
	p, err := branding.Save(slot, img, utils.SetRelayMetadataField)
	if err == nil {
		pruneBrandingImages()
	}
	brandingMu.Unlock()
	if err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 branding image stored", "signer", signer, "slot", slot,
		"from", from, "type", img.Type, "bytes", len(img.Data), "path", p)
	return map[string]any{
		"url":    p,
		"type":   img.Type,
		"width":  img.Width,
		"height": img.Height,
		"bytes":  len(img.Data),
	}, ""
}
