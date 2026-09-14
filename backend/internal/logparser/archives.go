package logparser

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ArchiveFile describes one rotated-out log archive produced by
// BackupAndPurge: eqlog_<Char>_pq.proj.<YYYY-MM-DD>.bak.zip (a single
// deflate entry holding the full log as of that date) or the legacy
// uncompressed .bak.txt form from an earlier version of the feature.
type ArchiveFile struct {
	Path       string    `json:"path"`
	ArchivedAt time.Time `json:"archived_at"` // from the date in the name; file mtime as fallback
	Compressed bool      `json:"compressed"`  // .bak.zip vs .bak.txt
	Bytes      int64     `json:"bytes"`       // uncompressed size (summed zip entry sizes, or file size)
}

// reArchiveName matches the "<YYYY-MM-DD>.bak.zip" / ".bak.txt" tail that
// BackupAndPurge appends to eqlog_<Char>_pq.proj.
var reArchiveName = regexp.MustCompile(`\.(\d{4}-\d{2}-\d{2})\.bak\.(?:zip|txt)$`)

// DiscoverArchives returns the log archives for one character found in
// eqPath, oldest first. Returns nil when eqPath or character is empty or no
// archives exist. A file that can't be stat'd — or a .bak.zip whose central
// directory can't be read — is skipped rather than failing the batch.
func DiscoverArchives(eqPath, character string) []ArchiveFile {
	if eqPath == "" || character == "" {
		return nil
	}
	prefix := "eqlog_" + character + "_pq.proj"
	var out []ArchiveFile
	for _, suffix := range []string{".*.bak.zip", ".*.bak.txt"} {
		matches, _ := filepath.Glob(filepath.Join(eqPath, prefix+suffix))
		for _, p := range matches {
			if af, ok := describeArchive(p); ok {
				out = append(out, af)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].ArchivedAt.Equal(out[j].ArchivedAt) {
			return out[i].ArchivedAt.Before(out[j].ArchivedAt)
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func describeArchive(path string) (ArchiveFile, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return ArchiveFile{}, false
	}
	af := ArchiveFile{Path: path, ArchivedAt: info.ModTime(), Bytes: info.Size()}
	if m := reArchiveName.FindStringSubmatch(filepath.Base(path)); m != nil {
		if d, err := time.ParseInLocation("2006-01-02", m[1], time.Local); err == nil {
			af.ArchivedAt = d
		}
	}
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		af.Compressed = true
		size, err := ArchiveUncompressedSize(path)
		if err != nil {
			return ArchiveFile{}, false
		}
		af.Bytes = size
	}
	return af, true
}

// ArchiveUncompressedSize returns the total uncompressed byte size of a
// .bak.zip archive (summed over every entry), for pre-sizing a scan's
// progress bar. For a non-zip path it returns the plain file size.
func ArchiveUncompressedSize(path string) (int64, error) {
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		fi, err := os.Stat(path)
		if err != nil {
			return 0, err
		}
		return fi.Size(), nil
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	var total int64
	for _, f := range r.File {
		total += int64(f.UncompressedSize64)
	}
	return total, nil
}

// OpenArchive opens an archive for streaming its log text. For a .bak.zip it
// returns a reader over the first entry whose name ends ".txt" (falling back
// to the first entry); for any other path it is a plain file open. Closing
// the returned reader releases the underlying file and, for a zip, its
// central-directory handle.
func OpenArchive(path string) (io.ReadCloser, error) {
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return os.Open(path)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	var entry *zip.File
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".txt") {
			entry = f
			break
		}
	}
	if entry == nil && len(zr.File) > 0 {
		entry = zr.File[0]
	}
	if entry == nil {
		zr.Close()
		return nil, fmt.Errorf("archive %s has no entries", filepath.Base(path))
	}
	rc, err := entry.Open()
	if err != nil {
		zr.Close()
		return nil, err
	}
	return &zipEntryReader{rc: rc, zr: zr}, nil
}

// zipEntryReader couples an open zip entry to its parent reader so a single
// Close tears both down.
type zipEntryReader struct {
	rc io.ReadCloser
	zr *zip.ReadCloser
}

func (z *zipEntryReader) Read(p []byte) (int, error) { return z.rc.Read(p) }

func (z *zipEntryReader) Close() error {
	err := z.rc.Close()
	if zerr := z.zr.Close(); err == nil {
		err = zerr
	}
	return err
}

// CompressLegacyArchive converts one legacy uncompressed .bak.txt archive
// (produced by an earlier version of Archive & Trim, before it moved to
// .bak.zip — see BackupAndPurge) into a single-entry .bak.zip in place, then
// removes the .txt. The zip entry is named after the live log
// ("eqlog_<Char>_pq.proj.txt") so OpenArchive reads it the same way as any
// other archive. Returns the new .bak.zip path.
func CompressLegacyArchive(path string) (string, error) {
	base := filepath.Base(path)
	m := reArchiveName.FindStringSubmatch(base)
	if m == nil || !strings.HasSuffix(strings.ToLower(base), ".bak.txt") {
		return "", fmt.Errorf("not a legacy .bak.txt archive: %s", base)
	}
	entryName := strings.TrimSuffix(base, m[0]) + ".txt"
	newPath := strings.TrimSuffix(path, ".txt") + ".zip"

	srcInfo, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat archive: %w", err)
	}
	if err := zipFile(path, newPath, entryName, srcInfo); err != nil {
		return "", fmt.Errorf("compress archive: %w", err)
	}
	if err := verifyZipEntry(newPath, entryName, srcInfo.Size()); err != nil {
		os.Remove(newPath)
		return "", fmt.Errorf("verify compressed archive: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return newPath, fmt.Errorf("compressed to %s but could not remove original: %w", filepath.Base(newPath), err)
	}
	return newPath, nil
}

// CompressResult is the outcome of compressing one legacy archive.
type CompressResult struct {
	OldPath string
	NewPath string
	Err     error
}

// CompressLegacyArchives compresses every legacy .bak.txt archive found for
// character in eqPath, oldest first. It keeps going after a per-file error
// so one bad archive doesn't block the rest.
func CompressLegacyArchives(eqPath, character string) []CompressResult {
	var out []CompressResult
	for _, af := range DiscoverArchives(eqPath, character) {
		if af.Compressed {
			continue
		}
		newPath, err := CompressLegacyArchive(af.Path)
		out = append(out, CompressResult{OldPath: af.Path, NewPath: newPath, Err: err})
	}
	return out
}
