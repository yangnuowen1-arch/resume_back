package handler

import (
	"testing"
)

func TestCollectJobDynamicFields(t *testing.T) {
	payload := []byte(`{
		"title": "Go Engineer",
		"department": "Platform",
		"dynamicFields": {"remote": true},
		"formData": {"interviewRounds": 3},
		"workYears": "3-5"
	}`)

	fields, err := collectJobDynamicFields(payload, map[string]interface{}{"remote": true})
	if err != nil {
		t.Fatalf("collect job dynamic fields: %v", err)
	}

	if fields["remote"] != true {
		t.Fatalf("expected dynamicFields.remote to be collected")
	}
	if fields["workYears"] != "3-5" {
		t.Fatalf("expected unknown top-level field to be collected")
	}
	if fields["interviewRounds"] != float64(3) {
		t.Fatalf("expected formData field to be collected")
	}
	if fields["department"] != "Platform" {
		t.Fatalf("expected non-basic field to be collected as dynamic field")
	}
}

func TestCollectJobDynamicFieldsKeepsEmptyDynamicFieldsExplicit(t *testing.T) {
	payload := []byte(`{
		"title": "Go Engineer",
		"status": "draft",
		"priority": "normal",
		"dynamicFields": {}
	}`)

	fields, err := collectJobDynamicFields(payload, map[string]interface{}{})
	if err != nil {
		t.Fatalf("collect job dynamic fields: %v", err)
	}

	if fields == nil {
		t.Fatalf("expected empty dynamicFields object to remain explicit")
	}
	if len(fields) != 0 {
		t.Fatalf("expected dynamicFields to be empty, got %v", fields)
	}
}
