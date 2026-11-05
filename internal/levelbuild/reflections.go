package levelbuild

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldreflection"
	"github.com/karty-game/karty/internal/assetpipeline"
)

func reflectionSource(module []byte) (level.Envelope, *world.Document, error) {
	envelope, err := level.DecodePassiveModule(module)
	if err != nil {
		return level.Envelope{}, nil, err
	}

	data, _, found := envelope.Read(world.EntryName, 0, level.MaxEntrySize)
	if !found {
		return level.Envelope{}, nil, worldreflection.ErrCubemaps
	}

	document, err := world.Decode(data)

	return envelope, &document, err
}

func appendSavedReflections(
	ctx context.Context,
	module []byte,
	features []string,
	directory string,
	assets *assetBuild,
	chunks []ChunkArtifact,
	required bool,
) ([]byte, []string, error) {
	if assets == nil || !slices.Contains(features, "world/fixed-lighting@1") {
		if required {
			return nil, nil, fmt.Errorf("required reflections need fixed offline lighting: %w", ErrManifest)
		}

		return module, features, nil
	}

	if assets.reflectionCapture {
		if preparedGPUReflectionEnabled(assets) && slices.Contains(features, worldgpuchunk.Feature) {
			return appendGPUReflectionCapturePlaceholder(ctx, module, features, assets, chunks)
		}

		return module, features, nil
	}

	relative, err := automaticBakePath(assets.projectRoot, directory)
	if err != nil {
		return nil, nil, err
	}

	envelope, document, err := reflectionSource(module)
	if err != nil {
		return nil, nil, err
	}

	key := worldreflection.InputDigest(envelope)

	data, capture, valid := readSavedReflectionCapture(
		assets.projectRoot,
		filepath.Join(relative, "reflections-"+key+".krc"),
		envelope,
		document,
	)

	if !valid {
		if required {
			return nil, nil, fmt.Errorf(
				"required reflection capture is missing, stale or invalid for %s; run karty render reflections %s: %w",
				directory,
				filepath.Base(directory),
				ErrManifest,
			)
		}

		return module, features, nil
	}

	if required {
		if err := validateRequiredReflectionCoverage(capture, document); err != nil {
			return nil, nil, err
		}
	}

	if !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.Capability(worldreflection.Feature)) {
		return nil, nil, fmt.Errorf("saved reflections require %s: %w", worldreflection.Feature, ErrManifest)
	}

	reflectionFeatures := []string{worldreflection.Feature}
	if slices.Contains(features, worldgpuchunk.Feature) &&
		slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.Capability(worldreflection.GPUFeature)) {
		var present bool

		data, present, err = prepareSavedGPUReflections(ctx, data, envelope, chunks, assets)
		if err != nil {
			return nil, nil, err
		}

		if !present {
			if required {
				return nil, nil, fmt.Errorf("required reflection capture has no environments: %w", ErrManifest)
			}

			return module, features, nil
		}

		reflectionFeatures = append(reflectionFeatures, worldreflection.GPUFeature)
	}

	return appendReflectionResource(envelope, features, data, reflectionFeatures)
}

func appendReflectionResource(envelope level.Envelope, features []string, data []byte,
	reflectionFeatures []string,
) ([]byte, []string, error) {
	entries := make([]level.SourceEntry, 0, len(envelope.Entries)+1)
	for _, entry := range envelope.Entries {
		bytes, _, _ := envelope.Read(entry.Name, 0, entry.Length)
		entries = append(entries, level.SourceEntry{Name: entry.Name, Kind: entry.Kind, Data: bytes})
	}

	entries = append(entries, level.SourceEntry{Name: worldreflection.EntryName, Kind: level.EntryData, Data: data})
	if _, err := completeWebGPUEnvelopeSize(envelope.Metadata, entries); err != nil {
		return nil, nil, err
	}

	module, err := encodeLevelModule("reflections", envelope.Metadata, entries)
	if err != nil {
		return nil, nil, err
	}

	features = append(slices.Clone(features), reflectionFeatures...)
	slices.Sort(features)

	return module, slices.Compact(features), nil
}

// SaveReflectionCapture verifies a producer's complete output against the exact
// level inputs, then atomically saves it in the confined private bake cache.
func SaveReflectionCapture(root string, artifact Artifact, data []byte) (string, error) {
	if !slices.Contains(artifact.Features, "world/fixed-lighting@1") ||
		!slices.Contains(artifact.Features, worldgpuchunk.Feature) {
		return "", worldreflection.ErrCubemaps
	}

	envelope, document, err := reflectionSource(artifact.Bytes)
	if err != nil {
		return "", err
	}

	bundle, err := worldreflection.Decode(data)
	if err != nil {
		return "", err
	}

	if err := bundle.ValidateSource(envelope, document); err != nil {
		return "", fmt.Errorf("reflection capture does not match level: %w", err)
	}

	directory, err := prepareBakeDirectory(root, artifact.SourceDirectory)
	if err != nil {
		return "", err
	}

	name := "reflections-" + bundle.InputSHA256 + ".krc"

	if err := atomicBakeWrite(directory, name, data); err != nil {
		return "", err
	}

	return filepath.Join(directory, name), nil
}

func readSavedReflectionCapture(
	root, path string,
	envelope level.Envelope,
	document *world.Document,
) ([]byte, worldreflection.Bundle, bool) {
	data, err := readConfinedFile(root, path, worldreflection.MaxEncodedSize)
	if err != nil {
		return nil, worldreflection.Bundle{}, false
	}

	bundle, err := worldreflection.Decode(data)
	if err != nil || bundle.ValidateSource(envelope, document) != nil {
		return nil, worldreflection.Bundle{}, false
	}

	return data, bundle, true
}

// Keep offline conversion separate from packaging without changing source bytes.
func prepareSavedGPUReflections(ctx context.Context, data []byte, envelope level.Envelope,
	chunks []ChunkArtifact, assets *assetBuild,
) ([]byte, bool, error) {
	capture, err := worldreflection.Decode(data)
	if err != nil {
		return nil, false, err
	}

	if len(capture.Probes) == 0 {
		return nil, false, nil
	}

	prepared, _, err := assetpipeline.ProcessReflectionGPU(ctx, assets.projectRoot, assets.manifest, capture)
	if err != nil {
		return nil, false, err
	}

	prepared.DrawProbes, err = buildReflectionDrawProbes(ctx, envelope, chunks, capture)
	if err != nil {
		return nil, false, err
	}

	encoded, err := worldreflection.EncodePrepared(prepared)
	if err != nil {
		return nil, false, err
	}

	return encoded, true, nil
}
