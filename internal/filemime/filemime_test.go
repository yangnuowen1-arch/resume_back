package filemime

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		filename    string
		want        string
	}{
		{
			name:        "bare pdf extension",
			contentType: "pdf",
			want:        "application/pdf",
		},
		{
			name:        "dotted uppercase pdf extension",
			contentType: ".PDF",
			want:        "application/pdf",
		},
		{
			name:        "full pdf media type with params",
			contentType: "Application/PDF; charset=binary",
			want:        "application/pdf",
		},
		{
			name:        "pdf alias",
			contentType: "application/x-pdf",
			want:        "application/pdf",
		},
		{
			name:        "docx filename",
			contentType: "application/octet-stream",
			filename:    "resume.docx",
			want:        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		},
		{
			name:        "preserve specific media type",
			contentType: "text/plain; charset=utf-8",
			filename:    "resume.pdf",
			want:        "text/plain",
		},
		{
			name:     "fallback from filename",
			filename: "resume.pdf",
			want:     "application/pdf",
		},
		{
			name:        "unknown extension",
			contentType: "unknown",
			filename:    "resume.unknown",
			want:        "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.contentType, tt.filename); got != tt.want {
				t.Fatalf("Normalize(%q, %q) = %q, want %q", tt.contentType, tt.filename, got, tt.want)
			}
		})
	}
}

func TestNormalizeAnyUsesSpecificFallbackBeforeExtension(t *testing.T) {
	got := NormalizeAny("resume", "application/octet-stream", "application/pdf")
	if got != "application/pdf" {
		t.Fatalf("NormalizeAny() = %q, want application/pdf", got)
	}
}
