package config

import "testing"

func TestMailboxEnabledRequiresGoogleOAuthAndPublicR2(t *testing.T) {
	configured := &Config{
		GoogleOAuthClientID:     "client-id",
		GoogleOAuthClientSecret: "client-secret",
		GoogleOAuthRedirectURL:  "https://api.example.com/callback",
		R2Endpoint:              "https://account.r2.cloudflarestorage.com",
		R2Bucket:                "resumes",
		R2AccessKeyID:           "access-key",
		R2SecretAccessKey:       "secret-key",
		R2PublicBaseURL:         "https://resume.example.com",
	}
	if !configured.MailboxEnabled() {
		t.Fatal("mailbox should be enabled when OAuth and public R2 are configured")
	}

	withoutPublicURL := *configured
	withoutPublicURL.R2PublicBaseURL = ""
	if withoutPublicURL.MailboxEnabled() {
		t.Fatal("mailbox must not use an r2:// URL that the candidate list cannot open")
	}

	withoutR2 := *configured
	withoutR2.R2Bucket = ""
	if withoutR2.MailboxEnabled() {
		t.Fatal("mailbox must not silently fall back to local storage")
	}
}
