package repository

import (
	"strings"
	"testing"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
)

func TestValidatePersistAttachmentInputBoundsIndexedIdentifiers(t *testing.T) {
	validMessage := MailboxMessageMetadata{AccountID: 1, MessageID: "message-1"}
	validAttachment := MailboxAttachmentMetadata{AttachmentKey: "attachment-1", AttachmentIndex: 0}

	if err := validatePersistAttachmentInput(validMessage, validAttachment, &model.Candidate{}, &model.Resume{}); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}

	tooLongMessage := validMessage
	tooLongMessage.MessageID = strings.Repeat("m", maxMailboxMessageIDLength+1)
	if err := validatePersistAttachmentInput(tooLongMessage, validAttachment, &model.Candidate{}, &model.Resume{}); err == nil {
		t.Fatal("oversized indexed message ID must be rejected")
	}

	tooLongAttachment := validAttachment
	tooLongAttachment.AttachmentKey = strings.Repeat("a", maxMailboxAttachmentKeyLength+1)
	if err := validatePersistAttachmentInput(validMessage, tooLongAttachment, &model.Candidate{}, &model.Resume{}); err == nil {
		t.Fatal("oversized indexed attachment key must be rejected")
	}
}
