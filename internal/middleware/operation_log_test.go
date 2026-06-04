package middleware

import (
	"net/http"
	"testing"
)

func TestOperationActionNamesResumeUpload(t *testing.T) {
	action := operationAction(http.MethodPost, "/candidates/:id/resume", "candidates")

	if action != "upload_resume" {
		t.Fatalf("expected upload_resume, got %q", action)
	}
}

func TestOperationActionNamesNestedUpdate(t *testing.T) {
	action := operationAction(http.MethodPut, "/users/:id/roles", "users")

	if action != "update_users_roles" {
		t.Fatalf("expected update_users_roles, got %q", action)
	}
}

func TestResponseDataID(t *testing.T) {
	responseBody := []byte(`{"code":0,"message":"success","data":{"id":42}}`)

	id := responseDataID(responseBody)
	if id == nil || *id != 42 {
		t.Fatalf("expected response data id 42, got %v", id)
	}
}
