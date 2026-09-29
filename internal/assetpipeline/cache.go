package assetpipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	CacheSchemaVersion       = 2
	MaxCacheOptionsBytes     = 32 * 1024
	MaxCacheMetadataBytes    = 64 * 1024
	MaxCachePayloadBytes     = 64 * 1024 * 1024
	cacheLockStaleAfter      = 2 * time.Minute
	cacheLockPollInterval    = 10 * time.Millisecond
	maxCacheKindLength       = 64
	maxProcessorLength       = 128
	maxRevisionCount         = 32
	maxRevisionNameLength    = 64
	maxRevisionVersionLength = 128
	maxEncodingLength        = 64
)

var (
	ErrCacheRecipe   = errors.New("asset cache recipe is invalid")
	ErrCacheOutput   = errors.New("asset processor output is invalid")
	errCacheCanceled = errors.New("asset cache operation canceled")
)

// Revision identifies one implementation detail that can change processed
// bytes, such as an importer, normalizer, filter, resampler, or codec.
type Revision struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Recipe is a canonical, immutable processor recipe. It deliberately excludes
// logical asset names, filesystem paths, and SDK versions.
type Recipe struct {
	kind      string
	processor string
	revisions []Revision
	options   []byte
	digest    string
}

type recipeDocument struct {
	Kind      string          `json:"kind"`
	Processor string          `json:"processor"`
	Revisions []Revision      `json:"revisions"`
	Options   json.RawMessage `json:"options"`
}

// NewRecipe canonicalizes options and revisions and computes their stable
// identity. Equivalent option maps produce the same digest regardless of map
// insertion order.
func NewRecipe(kind, processor string, revisions []Revision, options any) (Recipe, error) {
	if !validCacheToken(kind, maxCacheKindLength) || !validCacheToken(processor, maxProcessorLength) ||
		len(revisions) == 0 || len(revisions) > maxRevisionCount {
		return Recipe{}, ErrCacheRecipe
	}

	canonicalRevisions := slices.Clone(revisions)
	slices.SortFunc(canonicalRevisions, func(left, right Revision) int { return strings.Compare(left.Name, right.Name) })

	previous := ""
	for _, revision := range canonicalRevisions {
		if !validCacheToken(revision.Name, maxRevisionNameLength) ||
			!validCacheToken(revision.Version, maxRevisionVersionLength) || revision.Name == previous {
			return Recipe{}, ErrCacheRecipe
		}

		previous = revision.Name
	}

	canonicalOptions, err := canonicalJSON(options)
	if err != nil || len(canonicalOptions) == 0 || len(canonicalOptions) > MaxCacheOptionsBytes {
		return Recipe{}, ErrCacheRecipe
	}

	document := recipeDocument{Kind: kind, Processor: processor, Revisions: canonicalRevisions, Options: canonicalOptions}

	encoded, err := json.Marshal(document)
	if err != nil {
		return Recipe{}, ErrCacheRecipe
	}

	digest := sha256.Sum256(encoded)

	return Recipe{
		kind: kind, processor: processor, revisions: canonicalRevisions,
		options: canonicalOptions, digest: hex.EncodeToString(digest[:]),
	}, nil
}

func (recipe Recipe) Kind() string          { return recipe.kind }
func (recipe Recipe) Processor() string     { return recipe.processor }
func (recipe Recipe) Digest() string        { return recipe.digest }
func (recipe Recipe) Revisions() []Revision { return slices.Clone(recipe.revisions) }
func (recipe Recipe) OptionsJSON() []byte   { return bytes.Clone(recipe.options) }

// SourceSnapshot is an immutable, content-hashed source captured before a
// processor inspects it. Processors receive a copy through Bytes.
type SourceSnapshot struct {
	contents []byte
	digest   string
}

// SnapshotBytes copies and hashes in-memory source content.
func SnapshotBytes(contents []byte) (SourceSnapshot, error) {
	if len(contents) == 0 || len(contents) > MaxCachePayloadBytes {
		return SourceSnapshot{}, ErrCacheRecipe
	}

	copyOfContents := bytes.Clone(contents)
	digest := sha256.Sum256(copyOfContents)

	return SourceSnapshot{contents: copyOfContents, digest: hex.EncodeToString(digest[:])}, nil
}

// SnapshotFile reads and hashes a regular source file through one open handle;
// filesystem timestamps are never used as content identity.
func SnapshotFile(path string) (SourceSnapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return SourceSnapshot{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return SourceSnapshot{}, err
	}

	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > MaxCachePayloadBytes {
		return SourceSnapshot{}, ErrCacheRecipe
	}

	contents, err := io.ReadAll(io.LimitReader(file, MaxCachePayloadBytes+1))
	if err != nil {
		return SourceSnapshot{}, err
	}

	return SnapshotBytes(contents)
}

func (snapshot SourceSnapshot) Bytes() []byte  { return bytes.Clone(snapshot.contents) }
func (snapshot SourceSnapshot) Digest() string { return snapshot.digest }
func (snapshot SourceSnapshot) Size() int64    { return int64(len(snapshot.contents)) }

// Processed is the processor-neutral result published by Cache.Resolve.
// Metadata must be JSON-marshalable and is stored as bounded canonical JSON.
type Processed struct {
	Encoding string
	Payload  []byte
	Metadata any
}

// Processor transforms one immutable source snapshot according to a recipe.
type Processor func(context.Context, SourceSnapshot, Recipe) (Processed, error)

// Artifact is a validated immutable cache entry.
type Artifact struct {
	CacheKey     string
	RecipeDigest string
	SourceDigest string
	OutputDigest string
	Encoding     string
	PayloadPath  string
	EntryPath    string
	OutputBytes  int64
	Metadata     json.RawMessage
	CacheHit     bool
}

// Cache stores verified processed artifacts for one project.
type Cache struct {
	root string
}

// NewProjectCache opens the project-owned asset cache schema v2 namespace.
func NewProjectCache(projectRoot string) *Cache {
	return &Cache{root: filepath.Join(projectRoot, ".karty", "cache", "assets", "v2")}
}

type cacheEntry struct {
	Schema       int             `json:"schema"`
	CacheKey     string          `json:"cacheKey"`
	RecipeDigest string          `json:"recipeDigest"`
	SourceDigest string          `json:"sourceDigest"`
	OutputDigest string          `json:"outputDigest"`
	OutputBytes  int64           `json:"outputBytes"`
	Kind         string          `json:"kind"`
	Processor    string          `json:"processor"`
	Encoding     string          `json:"encoding"`
	Metadata     json.RawMessage `json:"metadata"`
}

// Resolve returns a verified warm entry or runs processor once under a
// cross-process lock and atomically publishes the validated result.
func (cache *Cache) Resolve(ctx context.Context, source SourceSnapshot, recipe Recipe, processor Processor) (Artifact, error) {
	if cache == nil || cache.root == "" || processor == nil || source.digest == "" || recipe.digest == "" {
		return Artifact{}, ErrCacheRecipe
	}

	if err := os.MkdirAll(cache.root, 0o750); err != nil {
		return Artifact{}, fmt.Errorf("create asset cache: %w", err)
	}

	key := cacheKey(source.digest, recipe.digest)

	entryPath := filepath.Join(cache.root, key)
	if artifact, valid := cache.load(entryPath, key, source, recipe); valid {
		artifact.CacheHit = true

		return artifact, nil
	}

	release, err := cache.acquire(ctx, key, entryPath, source, recipe)
	if err != nil {
		return Artifact{}, err
	}
	defer release()

	if artifact, valid := cache.load(entryPath, key, source, recipe); valid {
		artifact.CacheHit = true

		return artifact, nil
	}

	processed, err := processor(ctx, source, recipe)
	if err != nil {
		return Artifact{}, err
	}

	if err := ctx.Err(); err != nil {
		return Artifact{}, fmt.Errorf("%w: %w", errCacheCanceled, err)
	}

	entry, err := makeCacheEntry(key, source, recipe, processed)
	if err != nil {
		return Artifact{}, err
	}

	if err := cache.publish(entryPath, entry, processed.Payload); err != nil {
		return Artifact{}, err
	}

	artifact, valid := cache.load(entryPath, key, source, recipe)
	if !valid {
		return Artifact{}, ErrCacheOutput
	}

	artifact.CacheHit = false

	return artifact, nil
}

func makeCacheEntry(key string, source SourceSnapshot, recipe Recipe, processed Processed) (cacheEntry, error) {
	if !validCacheToken(processed.Encoding, maxEncodingLength) ||
		len(processed.Payload) == 0 || len(processed.Payload) > MaxCachePayloadBytes {
		return cacheEntry{}, ErrCacheOutput
	}

	metadata, err := canonicalJSON(processed.Metadata)
	if err != nil || len(metadata) > MaxCacheMetadataBytes/2 {
		return cacheEntry{}, ErrCacheOutput
	}

	outputDigest := sha256.Sum256(processed.Payload)

	return cacheEntry{
		Schema: CacheSchemaVersion, CacheKey: key, RecipeDigest: recipe.digest, SourceDigest: source.digest,
		OutputDigest: hex.EncodeToString(outputDigest[:]), OutputBytes: int64(len(processed.Payload)),
		Kind: recipe.kind, Processor: recipe.processor, Encoding: processed.Encoding, Metadata: metadata,
	}, nil
}

func (cache *Cache) load(path, key string, source SourceSnapshot, recipe Recipe) (Artifact, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Artifact{}, false
	}

	metadataPath := filepath.Join(path, "metadata.json")
	payloadPath := filepath.Join(path, "payload")

	metadataInfo, err := os.Lstat(metadataPath)
	if err != nil || !metadataInfo.Mode().IsRegular() || metadataInfo.Size() < 2 || metadataInfo.Size() > MaxCacheMetadataBytes {
		return Artifact{}, false
	}

	encoded, err := os.ReadFile(metadataPath)
	if err != nil {
		return Artifact{}, false
	}

	var entry cacheEntry

	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&entry); err != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		entry.Schema != CacheSchemaVersion || entry.CacheKey != key || entry.RecipeDigest != recipe.digest ||
		entry.SourceDigest != source.digest || entry.Kind != recipe.kind || entry.Processor != recipe.processor ||
		!validDigest(entry.OutputDigest) || !validCacheToken(entry.Encoding, maxEncodingLength) ||
		entry.OutputBytes < 1 || entry.OutputBytes > MaxCachePayloadBytes || len(entry.Metadata) > MaxCacheMetadataBytes/2 || !json.Valid(entry.Metadata) {
		return Artifact{}, false
	}

	payloadInfo, err := os.Lstat(payloadPath)
	if err != nil || !payloadInfo.Mode().IsRegular() || payloadInfo.Size() != entry.OutputBytes {
		return Artifact{}, false
	}

	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		return Artifact{}, false
	}

	digest := sha256.Sum256(payload)
	if hex.EncodeToString(digest[:]) != entry.OutputDigest {
		return Artifact{}, false
	}

	return Artifact{
		CacheKey: key, RecipeDigest: recipe.digest, SourceDigest: source.digest, OutputDigest: entry.OutputDigest,
		Encoding: entry.Encoding, PayloadPath: payloadPath, EntryPath: path, OutputBytes: entry.OutputBytes,
		Metadata: bytes.Clone(entry.Metadata),
	}, true
}

func (cache *Cache) publish(destination string, entry cacheEntry, payload []byte) error {
	temporary, err := os.MkdirTemp(cache.root, ".tmp-")
	if err != nil {
		return fmt.Errorf("create asset cache temporary entry: %w", err)
	}
	defer os.RemoveAll(temporary)

	metadata, err := json.Marshal(entry)
	if err != nil || len(metadata) > MaxCacheMetadataBytes {
		return ErrCacheOutput
	}

	if err := writeSyncedFile(filepath.Join(temporary, "payload"), payload); err != nil {
		return err
	}

	if err := writeSyncedFile(filepath.Join(temporary, "metadata.json"), append(metadata, '\n')); err != nil {
		return err
	}

	if err := syncDirectory(temporary); err != nil {
		return err
	}

	if _, err := os.Lstat(destination); err == nil {
		quarantine := destination + ".stale-" + filepath.Base(temporary)
		if err := os.Rename(destination, quarantine); err != nil {
			return fmt.Errorf("quarantine invalid cache entry: %w", err)
		}
		defer os.RemoveAll(quarantine)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("publish asset cache entry: %w", err)
	}

	if err := syncDirectory(cache.root); err != nil {
		return err
	}

	return nil
}

func (cache *Cache) acquire(ctx context.Context, key, entryPath string, source SourceSnapshot, recipe Recipe) (func(), error) {
	lockPath := filepath.Join(cache.root, ".lock-"+key)
	for {
		if err := os.Mkdir(lockPath, 0o700); err == nil {
			return func() { _ = os.RemoveAll(lockPath) }, nil
		} else if !os.IsExist(err) {
			return nil, fmt.Errorf("acquire asset cache lock: %w", err)
		}

		if _, valid := cache.load(entryPath, key, source, recipe); valid {
			return func() {}, nil
		}

		if info, err := os.Lstat(lockPath); err == nil && info.IsDir() && time.Since(info.ModTime()) > cacheLockStaleAfter {
			stale := lockPath + fmt.Sprintf(".stale-%d", time.Now().UnixNano())
			if os.Rename(lockPath, stale) == nil {
				_ = os.RemoveAll(stale)

				continue
			}
		}

		timer := time.NewTimer(cacheLockPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()

			return nil, fmt.Errorf("%w: %w", errCacheCanceled, ctx.Err())
		case <-timer.C:
		}
	}
}

func writeSyncedFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}

	if _, err := file.Write(contents); err != nil {
		file.Close()

		return err
	}

	if err := file.Sync(); err != nil {
		file.Close()

		return err
	}

	return file.Close()
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()

	return directory.Sync()
}

func cacheKey(sourceDigest, recipeDigest string) string {
	digest := sha256.Sum256([]byte("karty-asset-cache-v2\x00" + sourceDigest + "\x00" + recipeDigest))

	return hex.EncodeToString(digest[:])
}

func canonicalJSON(value any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}

	encoded, err := json.Marshal(value)
	if err != nil || !json.Valid(encoded) {
		return nil, ErrCacheRecipe
	}

	return encoded, nil
}

func validCacheToken(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum {
		return false
	}

	first := value[0]
	isInitial := first >= 'a' && first <= 'z' || first >= '0' && first <= '9'

	if !isInitial {
		return false
	}

	for _, character := range value[1:] {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' ||
			strings.ContainsRune("._+/@-", character) {
			continue
		}

		return false
	}

	return true
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}

	_, err := hex.DecodeString(value)

	return err == nil && strings.ToLower(value) == value
}
