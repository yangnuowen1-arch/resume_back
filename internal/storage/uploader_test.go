package storage

import "testing"

func TestNormalizeUploadContentTypeUsesPDFExtensionForGenericType(t *testing.T) {
	got := normalizeUploadContentType(
		"resumes/generated-key.pdf",
		"resume.pdf",
		"application/octet-stream",
	)
	if got != "application/pdf" {
		t.Fatalf("normalizeUploadContentType() = %q, want application/pdf", got)
	}
}

func TestNormalizeUploadContentTypeUsesObjectKeyExtension(t *testing.T) {
	got := normalizeUploadContentType(
		"resumes/generated-key.pdf",
		"",
		"application/octet-stream",
	)
	if got != "application/pdf" {
		t.Fatalf("normalizeUploadContentType() = %q, want application/pdf", got)
	}
}

func TestNormalizeUploadContentTypePreservesSpecificType(t *testing.T) {
	got := normalizeUploadContentType(
		"resumes/generated-key.pdf",
		"resume.pdf",
		"text/plain",
	)
	if got != "text/plain" {
		t.Fatalf("normalizeUploadContentType() = %q, want text/plain", got)
	}
}
