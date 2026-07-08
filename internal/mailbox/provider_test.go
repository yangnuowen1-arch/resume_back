package mailbox

import (
	"testing"
)

func TestAttachmentExt(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"resume.pdf", ".pdf"},
		{"Resume.PDF", ".pdf"},
		{"简历.docx", ".docx"},
		{"archive.tar.gz", ".gz"},
		{"noext", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := Attachment{Filename: c.filename}.Ext()
		if got != c.want {
			t.Errorf("Ext(%q) = %q, want %q", c.filename, got, c.want)
		}
	}
}

func TestAllowedExtSet(t *testing.T) {
	set := AllowedExtSet(" .PDF , docx ,, .txt")
	want := []string{".pdf", ".docx", ".txt"}
	if len(set) != len(want) {
		t.Fatalf("set size = %d, want %d (%v)", len(set), len(want), set)
	}
	for _, ext := range want {
		if _, ok := set[ext]; !ok {
			t.Errorf("expected %q in set, got %v", ext, set)
		}
	}
}

func TestFilterAttachments(t *testing.T) {
	allowed := AllowedExtSet(".pdf,.docx")
	attachments := []Attachment{
		{Filename: "a.pdf"},
		{Filename: "b.docx"},
		{Filename: "signature.png"}, // 签名档图片应被过滤
		{Filename: "c.PDF"},         // 大小写不敏感
		{Filename: "notes.txt"},     // 不在白名单
	}

	filtered := FilterAttachments(attachments, allowed)
	if len(filtered) != 3 {
		t.Fatalf("filtered len = %d, want 3 (%v)", len(filtered), filtered)
	}
	for _, att := range filtered {
		if att.Ext() != ".pdf" && att.Ext() != ".docx" {
			t.Errorf("unexpected attachment passed filter: %q", att.Filename)
		}
	}
}

func TestFilterAttachmentsEmptyWhitelist(t *testing.T) {
	// 空白名单不放行任何附件。
	filtered := FilterAttachments([]Attachment{{Filename: "a.pdf"}}, AllowedExtSet(""))
	if len(filtered) != 0 {
		t.Fatalf("expected 0 passed with empty whitelist, got %d", len(filtered))
	}
}

func TestParseFromHeader(t *testing.T) {
	cases := []struct {
		raw       string
		wantEmail string
		wantName  string
	}{
		{"张三 <ZhangSan@Example.com>", "zhangsan@example.com", "张三"},
		{"<bob@example.com>", "bob@example.com", ""},
		{"alice@example.com", "alice@example.com", ""},
		{`"Doe, John" <john@x.com>`, "john@x.com", "Doe, John"},
		{"  Spacey <spacey@x.com>  ", "spacey@x.com", "Spacey"},
		{"garbage-not-an-address", "garbage-not-an-address", ""},
	}
	for _, c := range cases {
		email, name := parseFromHeader(c.raw)
		if email != c.wantEmail {
			t.Errorf("parseFromHeader(%q) email = %q, want %q", c.raw, email, c.wantEmail)
		}
		if name != c.wantName {
			t.Errorf("parseFromHeader(%q) name = %q, want %q", c.raw, name, c.wantName)
		}
	}
}
