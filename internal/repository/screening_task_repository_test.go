package repository

import (
	"strings"
	"testing"
)

func TestScreeningTaskCandidateNameExpressionFallsBackToStoredAIOutput(t *testing.T) {
	if !strings.Contains(screeningTaskCandidateNameExpression, "candidates.name") {
		t.Fatal("candidate name expression must prefer the linked candidate name")
	}
	if !strings.Contains(screeningTaskCandidateNameExpression, "screening_results.candidate_name") {
		t.Fatal("candidate name expression must use the persisted screening result name")
	}
	if !strings.Contains(screeningTaskCandidateNameExpression, "#>> '{output,candidate_name}'") {
		t.Fatal("candidate name expression must read candidate_name from the stored screening output")
	}
	if !strings.Contains(screeningTaskListSelectColumns, screeningTaskCandidateNameExpression+" AS candidate_name") {
		t.Fatal("screening task list query must select the resolved candidate name")
	}
	if !strings.Contains(screeningTaskDetailSelectColumns, screeningTaskCandidateNameExpression+" AS candidate_name") {
		t.Fatal("screening task detail query must select the resolved candidate name")
	}
	for _, column := range []string{
		"screening_results.status",
		"screening_results.error_message",
		"screening_results.strengths",
		"screening_results.weaknesses",
		"screening_results.risks",
		"candidates.current_position AS candidate_current_title",
		"candidates.years_of_experience AS candidate_years_of_experience",
		"candidates.highest_education AS candidate_highest_education",
	} {
		if !strings.Contains(screeningTaskDetailSelectColumns, column) {
			t.Fatalf("screening task detail query must select %q", column)
		}
	}
}
