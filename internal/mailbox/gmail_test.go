package mailbox

import (
	"testing"

	gmailapi "google.golang.org/api/gmail/v1"
)

func TestParseGmailAttachmentQueryMessageMarksMetadataOnlyMessageAsHavingAttachment(t *testing.T) {
	metadataOnlyMessage := &gmailapi.Message{
		Id: "message-1",
		Payload: &gmailapi.MessagePart{Headers: []*gmailapi.MessagePartHeader{
			{Name: "From", Value: "杨诺雯 <1473018201@qq.com>"},
			{Name: "Subject", Value: "杨诺雯-AI全栈或应用开发"},
		}},
	}

	message := parseGmailAttachmentQueryMessage(metadataOnlyMessage)
	if !message.HasAttachments {
		t.Fatal("has:attachment 查询的 metadata 邮件必须进入附件下载流程")
	}
	if message.ID != "message-1" || message.FromEmail != "1473018201@qq.com" || message.FromName != "杨诺雯" {
		t.Fatalf("邮件元信息解析错误: %+v", message)
	}
}
