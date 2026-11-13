package branding

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
)

// webpSize reads a WebP's canvas size from its first chunk header, without
// decoding the image (the standard library has no WebP decoder).
func webpSize(data []byte) (w, h int, err error) {
	if len(data) < 30 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, errors.New("not a WebP file")
	}
	chunk := data[20:]
	switch string(data[12:16]) {
	case "VP8 ": // lossy: a 3-byte frame tag, the start code, then 14-bit sizes
		if !bytes.Equal(chunk[3:6], []byte{0x9d, 0x01, 0x2a}) {
			return 0, 0, errors.New("bad VP8 start code")
		}
		w = int(binary.LittleEndian.Uint16(chunk[6:8]) & 0x3fff)
		h = int(binary.LittleEndian.Uint16(chunk[8:10]) & 0x3fff)
	case "VP8L": // lossless: a signature byte, then width-1 and height-1 in 14 bits each
		if chunk[0] != 0x2f {
			return 0, 0, errors.New("bad VP8L signature")
		}
		bits := binary.LittleEndian.Uint32(chunk[1:5])
		w = int(bits&0x3fff) + 1
		h = int(bits>>14&0x3fff) + 1
	case "VP8X": // extended: flags and reserved bytes, then 24-bit canvas width-1 and height-1
		w = int(uint32(chunk[4])|uint32(chunk[5])<<8|uint32(chunk[6])<<16) + 1
		h = int(uint32(chunk[7])|uint32(chunk[8])<<8|uint32(chunk[9])<<16) + 1
	default:
		return 0, 0, errors.New("unknown WebP chunk")
	}
	return w, h, nil
}

// icoSize reads the largest image in an ICO. Each directory entry gives a
// size up to 256 (0 means 256); an entry stored as a PNG can be larger, so
// its own header is read instead.
func icoSize(data []byte) (w, h int, err error) {
	if len(data) < 6 {
		return 0, 0, errors.New("truncated ICO header")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count == 0 || len(data) < 6+16*count {
		return 0, 0, errors.New("truncated ICO directory")
	}
	for i := 0; i < count; i++ {
		e := data[6+16*i : 6+16*(i+1)]
		ew, eh := int(e[0]), int(e[1])
		if ew == 0 {
			ew = 256
		}
		if eh == 0 {
			eh = 256
		}
		size := binary.LittleEndian.Uint32(e[8:12])
		off := binary.LittleEndian.Uint32(e[12:16])
		if uint64(off)+uint64(size) > uint64(len(data)) {
			return 0, 0, errors.New("ICO entry past the end of the file")
		}
		if img := data[off : off+size]; bytes.HasPrefix(img, []byte("\x89PNG\r\n\x1a\n")) {
			cfg, _, err := image.DecodeConfig(bytes.NewReader(img))
			if err != nil {
				return 0, 0, err
			}
			ew, eh = cfg.Width, cfg.Height
		}
		w, h = max(w, ew), max(h, eh)
	}
	return w, h, nil
}
