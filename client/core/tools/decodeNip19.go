package tools

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/btcsuite/btcutil/bech32"
)

// Nip19Entity represents a decoded NIP-19 entity with TLV data
type Nip19Entity struct {
	Type   string   `json:"type"`   // npub, note, nprofile, nevent, naddr
	Data   string   `json:"data"`   // hex pubkey or event id
	Relays []string `json:"relays"` // optional relay hints
	Author string   `json:"author"` // optional author pubkey (for naddr/nevent)
	Kind   *int     `json:"kind"`   // optional event kind
	DTag   string   `json:"d_tag"`  // optional d tag (for naddr)
}

// DecodeNip19Entity decodes any NIP-19 bech32 encoded entity
func DecodeNip19Entity(encoded string) (*Nip19Entity, error) {
	log.ClientTools().Debug("Decoding NIP-19 entity", "encoded", encoded, "length", len(encoded))

	// Validate input
	if len(encoded) == 0 {
		return nil, errors.New("empty entity string")
	}

	// Basic format check
	if !strings.Contains(encoded, "1") {
		return nil, errors.New("invalid bech32 format - missing separator")
	}

	// Try standard decode first for shorter entities
	hrp, data, err := bech32.Decode(encoded)
	if err != nil {
		// If standard decode fails due to length, try custom decoding
		log.ClientTools().Debug("Standard bech32 decode failed, trying custom decode", "error", err)
		return decodeNip19Custom(encoded)
	}

	log.ClientTools().Debug("Bech32 decoded successfully", "hrp", hrp, "data_len", len(data))

	decodedData, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		log.ClientTools().Error("Failed to convert bits",
			"encoded", encoded,
			"hrp", hrp,
			"error", err)
		return nil, errors.New("failed to convert bech32 data: " + err.Error())
	}

	return processDecodedEntity(hrp, decodedData, encoded)
}

// decodeNip19Custom handles long NIP-19 entities that exceed btcsuite limits
func decodeNip19Custom(encoded string) (*Nip19Entity, error) {
	// Split at the '1' separator
	parts := strings.SplitN(encoded, "1", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid bech32 format")
	}

	hrp := parts[0]
	data := parts[1]

	log.ClientTools().Debug("Custom decoding", "hrp", hrp, "data_length", len(data))

	// Convert bech32 data manually
	decoded, err := customBech32Decode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bech32 data: %w", err)
	}

	// Convert 5-bit groups to 8-bit bytes
	decodedData, err := convertBits(decoded, 5, 8, false)
	if err != nil {
		return nil, fmt.Errorf("failed to convert bits: %w", err)
	}

	return processDecodedEntity(hrp, decodedData, encoded)
}

// customBech32Decode decodes bech32 data without length limits
func customBech32Decode(data string) ([]byte, error) {
	const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

	var decoded []byte
	for _, c := range data {
		pos := strings.IndexRune(charset, c)
		if pos == -1 {
			return nil, fmt.Errorf("invalid bech32 character: %c", c)
		}
		decoded = append(decoded, byte(pos))
	}

	// Remove the 6-byte checksum
	if len(decoded) < 6 {
		return nil, errors.New("data too short for checksum")
	}

	return decoded[:len(decoded)-6], nil
}

// convertBits converts between bit groups
func convertBits(data []byte, fromBits, toBits int, pad bool) ([]byte, error) {
	acc := 0
	bits := 0
	var result []byte
	maxv := (1 << toBits) - 1
	maxAcc := (1 << (fromBits + toBits - 1)) - 1

	for _, value := range data {
		if (int(value) >> fromBits) != 0 {
			return nil, errors.New("invalid data for base conversion")
		}
		acc = ((acc << fromBits) | int(value)) & maxAcc
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			result = append(result, byte((acc>>bits)&maxv))
		}
	}

	if pad {
		if bits > 0 {
			result = append(result, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, errors.New("invalid padding")
	}

	return result, nil
}

// processDecodedEntity processes decoded bech32 data into NIP-19 entity
func processDecodedEntity(hrp string, decodedData []byte, originalEncoded string) (*Nip19Entity, error) {
	log.ClientTools().Debug("Processing decoded entity", "hrp", hrp, "decoded_data_len", len(decodedData))

	entity := &Nip19Entity{
		Type: hrp,
	}

	switch hrp {
	case "npub":
		if len(decodedData) != 32 {
			return nil, errors.New("invalid npub length: expected 32 bytes, got " + fmt.Sprintf("%d", len(decodedData)))
		}
		entity.Data = strings.ToLower(hex.EncodeToString(decodedData))

	case "note":
		if len(decodedData) != 32 {
			return nil, errors.New("invalid note length: expected 32 bytes, got " + fmt.Sprintf("%d", len(decodedData)))
		}
		entity.Data = strings.ToLower(hex.EncodeToString(decodedData))

	case "nprofile", "nevent", "naddr":
		// Parse TLV data
		err := parseTLVData(entity, decodedData)
		if err != nil {
			log.ClientTools().Error("Failed to parse TLV data",
				"encoded", originalEncoded,
				"hrp", hrp,
				"decoded_data_len", len(decodedData),
				"error", err)
			return nil, errors.New("failed to parse TLV data: " + err.Error())
		}

	default:
		return nil, errors.New("unsupported bech32 prefix: " + hrp)
	}

	log.ClientTools().Debug("Successfully decoded NIP-19 entity",
		"type", entity.Type,
		"data", entity.Data,
		"relays_count", len(entity.Relays))

	return entity, nil
}

// parseTLVData parses Type-Length-Value encoded data for complex entities
func parseTLVData(entity *Nip19Entity, data []byte) error {
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			return errors.New("incomplete TLV data")
		}

		tlvType := data[offset]
		tlvLength := data[offset+1]
		offset += 2

		if offset+int(tlvLength) > len(data) {
			return errors.New("TLV length exceeds data")
		}

		value := data[offset : offset+int(tlvLength)]
		offset += int(tlvLength)

		switch tlvType {
		case 0: // special
			switch entity.Type {
			case "nprofile":
				if len(value) != 32 {
					return errors.New("invalid nprofile pubkey length")
				}
				entity.Data = strings.ToLower(hex.EncodeToString(value))
			case "nevent":
				if len(value) != 32 {
					return errors.New("invalid nevent event id length")
				}
				entity.Data = strings.ToLower(hex.EncodeToString(value))
			case "naddr":
				entity.DTag = string(value)
			}

		case 1: // relay
			relay := string(value)
			entity.Relays = append(entity.Relays, relay)

		case 2: // author
			if len(value) != 32 {
				return errors.New("invalid author pubkey length")
			}
			entity.Author = strings.ToLower(hex.EncodeToString(value))

		case 3: // kind
			if len(value) != 4 {
				return errors.New("invalid kind length")
			}
			kind := int(binary.BigEndian.Uint32(value))
			entity.Kind = &kind

		default:
			// Ignore unknown TLV types as per NIP-19
			log.ClientTools().Debug("Ignoring unknown TLV type", "type", tlvType)
		}
	}

	return nil
}

// DecodeNote decodes a note (event ID) from bech32
func DecodeNote(note string) (string, error) {
	log.ClientTools().Debug("Decoding note", "note", note)

	hrp, data, err := bech32.Decode(note)
	if err != nil {
		log.ClientTools().Error("Failed to decode bech32 note", "note", note, "error", err)
		return "", err
	}

	if hrp != "note" {
		log.ClientTools().Error("Invalid hrp in bech32 decode", "note", note, "hrp", hrp, "expected", "note")
		return "", errors.New("invalid hrp")
	}

	decodedData, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		log.ClientTools().Error("Failed to convert bits", "note", note, "error", err)
		return "", err
	}

	if len(decodedData) != 32 {
		log.ClientTools().Error("Invalid decoded note length", "note", note, "length", len(decodedData), "expected", 32)
		return "", errors.New("invalid event ID length")
	}

	eventId := strings.ToLower(hex.EncodeToString(decodedData))
	log.ClientTools().Debug("Successfully decoded note",
		"note", note,
		"event_id", eventId)

	return eventId, nil
}
