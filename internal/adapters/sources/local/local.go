// Package local is the local-folder source provider
// (backend-source-adapter.md FR-3, FR-7, FR-12): it lists the ebook
// files in a configured directory, treating every filename the OS
// returns as a string an attacker chose (CLAUDE.md). A file whose
// real, symlink-resolved path escapes the configured root is silently
// omitted, never surfaced as an error.
package local

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

// supportedExtensions is the closed set of ebook formats this phase
// lists. Anything else in the directory is skipped, not surfaced.
var supportedExtensions = map[string]string{
	".epub": "EPUB",
	".pdf":  "PDF",
	".mobi": "MOBI",
	".azw3": "AZW3",
	".cbz":  "CBZ",
	".cbr":  "CBR",
	".fb2":  "FB2",
	".djvu": "DJVU",
}

// Provider lists a single configured directory. It recurses into no
// subdirectory this phase (FR-7 Open questions).
type Provider struct {
	sourceID string
	basePath string
	codec    *sources.CursorCodec
	logger   *slog.Logger
}

var _ sources.Provider = (*Provider)(nil)

// New constructs a Provider. basePath's shape is validated here
// (FR-3); its existence and readability are a Probe concern, since an
// unmounted drive may be back later.
func New(sourceID, basePath string, codec *sources.CursorCodec, logger *slog.Logger) (*Provider, error) {
	if err := sources.ValidateLocalFolderPath(basePath); err != nil {
		return nil, err
	}
	return &Provider{sourceID: sourceID, basePath: basePath, codec: codec, logger: logger}, nil
}

// Probe checks the directory exists, is a directory, and is readable
// (FR-6). Capabilities for a reachable local folder are fixed: list and
// download yes, search no (FR-5).
func (p *Provider) Probe(_ context.Context) sources.ProbeResult {
	unreachable := func(detail string) sources.ProbeResult {
		return sources.ProbeResult{Status: sources.HealthUnreachable, Detail: detail}
	}

	info, err := os.Stat(p.basePath)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return unreachable(sources.DetailPathNotReadable)
		}
		return unreachable(sources.DetailPathNotFound)
	}
	if !info.IsDir() {
		return unreachable(sources.DetailPathNotFound)
	}

	f, err := os.Open(p.basePath)
	if err != nil {
		return unreachable(sources.DetailPathNotReadable)
	}
	_, err = f.ReadDir(1)
	_ = f.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		return unreachable(sources.DetailPathNotReadable)
	}

	return sources.ProbeResult{
		Status:       sources.HealthReachable,
		Capabilities: domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true},
	}
}

// List returns one page of the directory's ebook files, sorted by
// filename, resuming after the cursor's last-seen name (FR-7). A file
// whose resolved path lies outside the configured root, or a broken
// symlink, is omitted from the page entirely (FR-12).
func (p *Provider) List(_ context.Context, cursor string, limit int) (sources.CandidatePage, error) {
	limit = sources.ClampLimit(limit)

	var after string
	if cursor != "" {
		c, err := p.codec.Decode(cursor, p.sourceID)
		if err != nil {
			return sources.CandidatePage{}, err
		}
		if c.Kind != sources.KindLocalFolder {
			return sources.CandidatePage{}, &domain.Error{Category: domain.InvalidInput, Message: "cursor is invalid"}
		}
		after = c.Position
	}

	realBase, err := filepath.EvalSymlinks(p.basePath)
	if err != nil {
		return sources.CandidatePage{}, &domain.Error{Category: domain.Unavailable, Message: "source is unavailable right now"}
	}

	entries, err := os.ReadDir(realBase)
	if err != nil {
		return sources.CandidatePage{}, &domain.Error{Category: domain.Unavailable, Message: "source is unavailable right now"}
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	items := make([]sources.SourceCandidate, 0, limit)
	var lastName string
	truncated := false

	for _, name := range names {
		if after != "" && name <= after {
			continue
		}
		if !p.isSupported(name) {
			continue
		}
		cand, ok := p.candidateFor(realBase, name)
		if !ok {
			continue
		}
		if len(items) == limit {
			truncated = true
			break
		}
		items = append(items, cand)
		lastName = name
	}

	page := sources.CandidatePage{Items: items}
	if truncated && lastName != "" {
		next := p.codec.Encode(sources.Cursor{
			SourceID: p.sourceID, Kind: sources.KindLocalFolder, Position: lastName,
		})
		page.NextCursor = &next
	}
	return page, nil
}

func (p *Provider) isSupported(name string) bool {
	_, ok := supportedExtensions[strings.ToLower(filepath.Ext(name))]
	return ok
}

// candidateFor builds a SourceCandidate for name, or returns ok=false if
// name must be skipped: a path-shaped name, a name whose resolved path
// escapes realBase, a broken symlink, or a non-regular file.
func (p *Provider) candidateFor(realBase, name string) (sources.SourceCandidate, bool) {
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, os.PathSeparator) || strings.ContainsRune(name, '/') {
		p.warn("skipped a directory entry with a path-shaped name")
		return sources.SourceCandidate{}, false
	}

	candidate := filepath.Join(realBase, name)
	real, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		p.warn("skipped a directory entry whose path did not resolve (broken symlink)")
		return sources.SourceCandidate{}, false
	}
	if !withinRoot(realBase, real) {
		p.warn("skipped a directory entry whose resolved path escapes the configured folder")
		return sources.SourceCandidate{}, false
	}

	info, err := os.Stat(real)
	if err != nil || !info.Mode().IsRegular() {
		return sources.SourceCandidate{}, false
	}

	format := supportedExtensions[strings.ToLower(filepath.Ext(name))]
	size := info.Size()
	ref, err := domain.NewFileReference(name, format, &size)
	if err != nil {
		return sources.SourceCandidate{}, false
	}
	return sources.SourceCandidate{Title: titleFromFilename(name), FileReference: ref}, true
}

// withinRoot reports whether real is root itself or lies beneath it,
// comparing resolved paths (FR-12 — filepath.Clean alone is not enough).
func withinRoot(root, real string) bool {
	if real == root {
		return true
	}
	rel, err := filepath.Rel(root, real)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// titleFromFilename is a best-effort display title — the filename minus
// its extension, underscores to spaces. The real title comes from
// phase 10's matching; this is only a candidate guess (FR-9).
func titleFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.TrimSpace(base)
	if base == "" {
		return name
	}
	return base
}

// Search is not supported for a local folder (FR-5 / FR-8).
func (p *Provider) Search(_ context.Context, _, _ string, _ int) (sources.CandidatePage, error) {
	return sources.CandidatePage{}, &domain.Error{Category: domain.Conflict, Message: "this source does not support search"}
}

// Resolve opens the bytes behind ref (FR-15, phase 10). The reference id
// is treated as opaque and re-checked for traversal safety before any
// file operation.
func (p *Provider) Resolve(_ context.Context, ref domain.FileReference) (io.ReadCloser, error) {
	name := ref.ReferenceID
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, os.PathSeparator) || strings.ContainsRune(name, '/') {
		return nil, &domain.Error{Category: domain.InvalidInput, Message: "file reference is not valid for this source"}
	}
	// Only book files are resolvable — the same extension filter List
	// applies. A request for a .env, .ssh key, or any other file the
	// listing would never surface is treated as not found, no oracle
	// (#176).
	if !p.isSupported(name) {
		return nil, &domain.Error{Category: domain.NotFound, Message: "file not found in this source"}
	}
	realBase, err := filepath.EvalSymlinks(p.basePath)
	if err != nil {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "source is unavailable right now"}
	}
	real, err := filepath.EvalSymlinks(filepath.Join(realBase, name))
	if err != nil || !withinRoot(realBase, real) {
		return nil, &domain.Error{Category: domain.NotFound, Message: "file not found in this source"}
	}
	info, err := os.Stat(real)
	if err != nil || !info.Mode().IsRegular() {
		return nil, &domain.Error{Category: domain.NotFound, Message: "file not found in this source"}
	}
	f, err := os.Open(real)
	if err != nil {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "source is unavailable right now"}
	}
	return f, nil
}

func (p *Provider) warn(msg string) {
	if p.logger != nil {
		p.logger.Warn(msg, slog.String("sourceId", p.sourceID))
	}
}
