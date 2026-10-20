package devserver

import (
	"context"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-ui/schema"
)

const uiHeaderBytes = 12
const maxUIBytes = uiHeaderBytes + 2*cartridge.MaxAssetBundle

type UIResource struct {
	Artifact      string `json:"artifact"`
	ContentSHA256 string `json:"contentSha256"`
	Size          uint64 `json:"size"`
}

func embeddedAssets(ctx context.Context, wasm []byte, section string) ([]cartridge.Asset, error) {
	data, _, err := customSection(ctx, wasm, section)
	if errors.Is(err, cartridge.ErrSectionMissing) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return cartridge.DecodeAssets(data)
}

func (snapshot *Snapshot) captureUI(ctx context.Context) error {
	templates, err := embeddedAssets(ctx, snapshot.files["game.kart"], ui.SectionName)
	if err != nil {
		return err
	}

	assets, err := embeddedAssets(ctx, snapshot.files["game.kart"], cartridge.AssetSectionName)
	if err != nil {
		return err
	}

	fonts := make([]cartridge.Asset, 0, 3)

	for _, entry := range assets {
		if err := ctx.Err(); err != nil {
			return err
		}

		if role, found := strings.CutPrefix(entry.Name, ui.FontAssetPrefix); found {
			if role != "body" && role != "display" && role != "mono" {
				return cartridge.ErrAssets
			}

			fonts = append(fonts, entry)
		}
	}

	encoded, err := encodeUIResource(ctx, templates, fonts)
	if err != nil {
		return err
	}

	if len(encoded) > maxUIBytes || snapshot.size+int64(len(encoded)) > maxSnapshotBytes || len(snapshot.files) >= maxSnapshotFiles {
		return ErrSnapshot
	}

	checksum, err := digestContext(ctx, encoded)
	if err != nil {
		return err
	}

	artifact := "__karty_dev/ui/" + checksum + ".bin"
	snapshot.ui = UIResource{Artifact: artifact, ContentSHA256: checksum, Size: uint64(len(encoded))}
	snapshot.files[artifact] = encoded
	snapshot.checksums[artifact] = checksum
	snapshot.size += int64(len(encoded))

	return nil
}

func normalizeUIRuntime(ctx context.Context, wasm []byte) ([]byte, error) {
	if _, _, err := manifestSection(ctx, wasm); err != nil {
		return nil, err
	}

	result := make([]byte, 0, len(wasm))
	result = append(result, wasm[:8]...)

	nonfonts, err := cartridge.EncodeAssets(nil)
	if err != nil {
		return nil, err
	}

	emptyUI := nonfonts

	for offset := 8; offset < len(wasm); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		start := offset
		kind := wasm[offset]
		offset++
		length, count := binary.Uvarint(wasm[offset:])
		offset += count
		end := offset + int(length)
		keep := true

		if kind == 0 {
			name, normalized, decodeErr := normalizedUISection(wasm[offset:end])
			if decodeErr != nil {
				return nil, decodeErr
			}

			if name == cartridge.AssetSectionName {
				nonfonts = normalized
			}

			keep = normalized == nil
		}

		if keep {
			result, err = appendUIBytes(ctx, result, wasm[start:end])
			if err != nil {
				return nil, err
			}
		}

		offset = end
	}

	for _, section := range []struct {
		name    string
		payload []byte
	}{{cartridge.AssetSectionName, nonfonts}, {ui.SectionName, emptyUI}} {
		payload := binary.AppendUvarint(nil, uint64(len(section.name)))
		payload = append(payload, section.name...)
		payload = append(payload, section.payload...)
		result = append(result, 0)
		result = binary.AppendUvarint(result, uint64(len(payload)))

		result, err = appendUIBytes(ctx, result, payload)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func appendUIBytes(ctx context.Context, destination, source []byte) ([]byte, error) {
	for len(source) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		chunk := min(contextChunkBytes, len(source))
		destination = append(destination, source[:chunk]...)
		source = source[chunk:]
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return destination, nil
}

func normalizedUIAssets(payload []byte, name string) ([]byte, error) {
	assets, err := cartridge.DecodeAssets(payload)
	if err != nil {
		return nil, err
	}

	kept := make([]cartridge.Asset, 0, len(assets))
	if name == cartridge.AssetSectionName {
		for _, entry := range assets {
			if !strings.HasPrefix(entry.Name, ui.FontAssetPrefix) {
				kept = append(kept, entry)
			}
		}
	}

	return cartridge.EncodeAssets(kept)
}

func normalizedUISection(section []byte) (string, []byte, error) {
	nameLength, nameCount := binary.Uvarint(section)
	content := nameCount + int(nameLength)

	name := string(section[nameCount:content])
	if name != ui.SectionName && name != cartridge.AssetSectionName {
		return "", nil, nil
	}

	normalized, err := normalizedUIAssets(section[content:], name)

	return name, normalized, err
}

func encodeUIResource(ctx context.Context, templates, fonts []cartridge.Asset) ([]byte, error) {
	templateBytes, err := cartridge.EncodeAssets(templates)
	if err != nil {
		return nil, err
	}

	fontBytes, err := cartridge.EncodeAssets(fonts)
	if err != nil {
		return nil, err
	}

	encoded := make([]byte, uiHeaderBytes, uiHeaderBytes+len(templateBytes)+len(fontBytes))
	copy(encoded, "KDU1")
	binary.LittleEndian.PutUint32(encoded[4:8], uint32(len(templateBytes)))
	binary.LittleEndian.PutUint32(encoded[8:12], uint32(len(fontBytes)))

	encoded, err = appendUIBytes(ctx, encoded, templateBytes)
	if err != nil {
		return nil, err
	}

	return appendUIBytes(ctx, encoded, fontBytes)
}
