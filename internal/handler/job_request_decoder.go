package handler

import (
	"encoding/json"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
)

func bindCreateJobRequest(c *gin.Context, req *dto.CreateJobRequest) error {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}

	type alias dto.CreateJobRequest
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*req = dto.CreateJobRequest(decoded)
	req.DynamicFields, err = collectJobDynamicFields(data, req.DynamicFields)
	return err
}

func bindUpdateJobRequest(c *gin.Context, req *dto.UpdateJobRequest) error {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}

	type alias dto.UpdateJobRequest
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*req = dto.UpdateJobRequest(decoded)
	req.DynamicFields, err = collectJobDynamicFields(data, req.DynamicFields)
	return err
}

func collectJobDynamicFields(data []byte, initial map[string]interface{}) (map[string]interface{}, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	explicit := false
	result := make(map[string]interface{}, len(initial))
	for key, value := range initial {
		result[key] = value
	}

	for _, alias := range []string{"dynamicFields", "formData"} {
		rawValue, ok := raw[alias]
		if !ok {
			continue
		}
		explicit = true

		var fields map[string]interface{}
		if err := json.Unmarshal(rawValue, &fields); err == nil {
			for key, value := range fields {
				result[key] = value
			}
		}
	}

	for key, rawValue := range raw {
		if _, ok := knownJobJSONFields[key]; ok {
			continue
		}

		var value interface{}
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}

	if len(result) == 0 && !explicit {
		return nil, nil
	}

	return result, nil
}

var knownJobJSONFields = map[string]struct{}{
	"categoryId":       {},
	"title":            {},
	"headcount":        {},
	"salaryMin":        {},
	"salaryMax":        {},
	"salaryMonths":     {},
	"experienceMin":    {},
	"experienceMax":    {},
	"description":      {},
	"responsibilities": {},
	"requirements":     {},
	"bonusPoints":      {},
	"status":           {},
	"priority":         {},
	"ownerUserId":      {},
	"tagIds":           {},
	"dynamicFields":    {},
	"formData":         {},
}
