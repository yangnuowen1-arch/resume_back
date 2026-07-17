package filemime

import (
	"mime"
	"path/filepath"
	"strings"
)

const OctetStream = "application/octet-stream"

var extensionContentTypes = map[string]string{
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".pdf":  "application/pdf",
}

var mediaTypeAliases = map[string]string{
	"application/acrobat": "application/pdf",
	"application/x-pdf":   "application/pdf",
}

// Normalize returns a concrete MIME type suitable for storing and forwarding to
// APIs that reject bare extensions such as "pdf".
func Normalize(contentType, filename string) string {
	return NormalizeAny(filename, contentType)
}

// NormalizeAny returns the first specific MIME type from multiple candidate
// content-type values, then falls back to the filename extension.
func NormalizeAny(filename string, contentTypes ...string) string {
	genericMediaType := ""
	for _, contentType := range contentTypes {
		if mediaType := normalizeMediaType(contentType); mediaType != "" {
			if alias := mediaTypeAliases[mediaType]; alias != "" {
				return alias
			}
			if !isGenericMediaType(mediaType) {
				return mediaType
			}
			if genericMediaType == "" {
				genericMediaType = mediaType
			}
			continue
		}

		if mediaType := normalizeExtension(contentType); mediaType != "" {
			return mediaType
		}
	}
	if mediaType := normalizeExtension(filepath.Ext(filename)); mediaType != "" {
		return mediaType
	}
	if genericMediaType != "" {
		return genericMediaType
	}
	return OctetStream
}

func normalizeMediaType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.Contains(value, "/") {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	typ, sub, ok := strings.Cut(mediaType, "/")
	if !ok || typ == "" || sub == "" {
		return ""
	}
	return mediaType
}

func normalizeExtension(value string) string {
	ext := strings.ToLower(strings.TrimSpace(value))
	if ext == "" || strings.Contains(ext, "/") {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if mediaType := extensionContentTypes[ext]; mediaType != "" {
		return mediaType
	}
	if mediaType := mime.TypeByExtension(ext); mediaType != "" {
		return normalizeMediaType(mediaType)
	}
	return ""
}

func isGenericMediaType(mediaType string) bool {
	switch mediaType {
	case OctetStream, "binary/octet-stream", "application/zip":
		return true
	default:
		return false
	}
}
