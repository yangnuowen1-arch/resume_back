package dify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// 前两次返回 503，第三次返回正常结果，验证会自动重试到成功。
func TestRunResumeScreening_RetriesOnServerError(t *testing.T) {
	var workflowCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/files/upload"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"file-123"}`))
		case strings.HasSuffix(r.URL.Path, "/workflows/run"):
			n := atomic.AddInt32(&workflowCalls, 1)
			if n < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"busy"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"workflow_run_id":"run-1","task_id":"task-1","data":{"status":"succeeded","outputs":{"screening_result":"ok"}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, APIKey: "test-key"})

	resp, err := client.RunResumeScreening(context.Background(), RunResumeScreeningRequest{
		File:     strings.NewReader("dummy resume"),
		Filename: "resume.pdf",
	})
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if resp.ResultText != "ok" {
		t.Fatalf("unexpected result text: %q", resp.ResultText)
	}
	if got := atomic.LoadInt32(&workflowCalls); got != 3 {
		t.Fatalf("expected 3 workflow attempts, got %d", got)
	}
}

// 返回 400 时不应重试，应直接失败。
func TestRunResumeScreening_NoRetryOnClientError(t *testing.T) {
	var workflowCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/files/upload"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"file-123"}`))
		case strings.HasSuffix(r.URL.Path, "/workflows/run"):
			atomic.AddInt32(&workflowCalls, 1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad input"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, APIKey: "test-key"})

	_, err := client.RunResumeScreening(context.Background(), RunResumeScreeningRequest{
		File:     strings.NewReader("dummy resume"),
		Filename: "resume.pdf",
	})
	if err == nil {
		t.Fatal("expected error on 400, got nil")
	}
	if got := atomic.LoadInt32(&workflowCalls); got != 1 {
		t.Fatalf("expected 1 workflow attempt (no retry), got %d", got)
	}
}
