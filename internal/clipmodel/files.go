package clipmodel

import (
	"net/url"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	FileListFormat = "application/x-mygo-file-list"
	URIListFormat  = "text/uri-list"
	// These contain exact native payloads. FileListFormat separately keeps the
	// path URLs resolved at capture, without replacing the original bytes.
	NativeFileURLFormat   = "application/x-copiggy-native-file-url"
	NativeFilenamesFormat = "application/x-copiggy-native-filenames"
	MaxFileReferences     = 64
)

// FileReference derives path text from a saved native representation. The same
// shape is retained for validating older history annotations during migration.
type FileReference struct {
	URL  string `json:"url"`
	Path string `json:"path"`
	Name string `json:"name"`
}

func parseFileReference(raw string) (FileReference, bool) {
	if raw == "" || len(raw) > 64<<10 || !utf8.ValidString(raw) {
		return FileReference{}, false
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "file") || u.Opaque != "" || u.User != nil ||
		(u.Host != "" && !strings.EqualFold(u.Host, "localhost")) || !utf8.ValidString(u.Path) || u.RawQuery != "" || u.ForceQuery ||
		u.Fragment != "" || u.RawFragment != "" || !strings.HasPrefix(u.Path, "/") ||
		strings.HasPrefix(u.Path, "//") || strings.ContainsRune(u.Path, 0) || strings.HasPrefix(u.Path, "/.file/id=") {
		return FileReference{}, false
	}
	return FileReference{URL: raw, Path: u.Path, Name: path.Base(u.Path)}, true
}

// FilesFromRepresentations recognizes declared file payloads before incidental
// filename text. It never stats files, resolves symlinks, or consumes promises.
// A mixed URI list remains a URI list; no subset silently becomes a file copy.
func FilesFromRepresentations(data Representations) ([]FileReference, error) {
	var files []FileReference
	for _, item := range data {
		var encoded []byte
		declared, single, found := false, false, false
		for _, format := range []string{FileListFormat, URIListFormat, NativeFileURLFormat} {
			for _, rep := range item {
				if rep.Format == format {
					encoded, declared, single = rep.Bytes, format != URIListFormat, format == NativeFileURLFormat
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			continue
		}
		lines := []string{string(encoded)}
		if !single {
			lines = strings.Split(strings.ReplaceAll(string(encoded), "\r\n", "\n"), "\n")
		}
		var itemFiles []FileReference
		mixed := false
		for _, line := range lines {
			if !single && (line == "" || strings.HasPrefix(line, "#")) {
				continue
			}
			file, ok := parseFileReference(line)
			if !ok {
				if declared {
					return nil, ContentError("Unsupported clipboard content")
				}
				mixed = true
				break
			}
			itemFiles = append(itemFiles, file)
		}
		if mixed {
			continue
		}
		if len(itemFiles) == 0 && declared {
			return nil, ContentError("Unsupported clipboard content")
		}
		files = append(files, itemFiles...)
		if len(files) > MaxFileReferences {
			return nil, ContentError("Unsupported clipboard content")
		}
	}
	return files, nil
}

func normalizedFiles(files []FileReference) ([]FileReference, bool) {
	if len(files) == 0 || len(files) > MaxFileReferences {
		return nil, false
	}
	for _, file := range files {
		parsed, ok := parseFileReference(file.URL)
		if !ok || parsed != file {
			return nil, false
		}
	}
	return slices.Clone(files), true
}

func fileNames(files []FileReference) string {
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = file.Name
	}
	return strings.Join(names, "\n")
}

// FilePathsText supplies the ordinary display and copy text for native files.
// It does not access the filesystem or modify the saved representations.
func FilePathsText(files []FileReference) (string, error) {
	files, ok := normalizedFiles(files)
	if !ok {
		return "", ErrInvalidCopyFormat
	}
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return strings.Join(paths, "\n"), nil
}
