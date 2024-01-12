package workflows

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/api/util"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/internal/workflow/match"
)

func workflowToDto(model *workflow.Workflow) gen.Workflow {
	return gen.Workflow{
		Id:        model.ID,
		Label:     model.Label,
		Enabled:   model.Enabled,
		Criteria:  util.ApplyConversion(model.Criteria, criteriaToDto),
		TargetIds: util.ApplyConversion(model.Targets, getTargetId),
	}
}

func criteriaToDto(criteria match.Criteria) gen.WorkflowCriteria {
	return gen.WorkflowCriteria{
		CombineType: criteriaCombineTypeToDto(criteria.CombineType),
		Key:         criteriaKeyToDto(criteria.Key),
		Type:        criteriaTypeToDto(criteria.Type),
		Value:       criteria.Value,
	}
}

func criteriaCombineTypeToDto(combineType match.CombineType) gen.WorkflowCriteriaCombineType {
	switch combineType {
	case match.AND:
		return gen.AND
	case match.OR:
		return gen.OR
	}

	panic("unreachable")
}

func criteriaKeyToDto(key match.Key) gen.WorkflowCriteriaKey {
	switch key {
	case match.TITLE:
		return gen.TITLE
	case match.RESOLUTION:
		return gen.RESOLUTION
	case match.SEASON_NUMBER:
		return gen.SEASONNUMBER
	case match.EPISODE_NUMBER:
		return gen.EPISODENUMBER
	case match.SOURCE_PATH:
		return gen.SOURCEPATH
	case match.SOURCE_NAME:
		return gen.SOURCENAME
	case match.SOURCE_EXTENSION:
		return gen.SOURCEEXTENSION
	}

	panic("unreachable")
}

func criteriaTypeToDto(t match.Type) gen.WorkflowCriteriaType {
	switch t {
	case match.EQUALS:
		return gen.EQUALS
	case match.NOT_EQUALS:
		return gen.NOTEQUALS
	case match.MATCHES:
		return gen.MATCHES
	case match.DOES_NOT_MATCH:
		return gen.DOESNOTMATCH
	case match.LESS_THAN:
		return gen.LESSTHAN
	case match.GREATER_THAN:
		return gen.GREATERTHAN
	case match.IS_PRESENT:
		return gen.ISPRESENT
	case match.IS_NOT_PRESENT:
		return gen.ISNOTPRESENT
	}

	panic("unreachable")
}

func criteriaCombineTypeToModel(combineType gen.WorkflowCriteriaCombineType) match.CombineType {
	switch combineType {
	case gen.AND:
		return match.AND
	case gen.OR:
		return match.OR
	}

	panic("unreachable")
}

func criteriaKeyToModel(key gen.WorkflowCriteriaKey) match.Key {
	switch key {
	case gen.TITLE:
		return match.TITLE
	case gen.RESOLUTION:
		return match.RESOLUTION
	case gen.SEASONNUMBER:
		return match.SEASON_NUMBER
	case gen.EPISODENUMBER:
		return match.EPISODE_NUMBER
	case gen.SOURCEPATH:
		return match.SOURCE_PATH
	case gen.SOURCENAME:
		return match.SOURCE_NAME
	case gen.SOURCEEXTENSION:
		return match.SOURCE_EXTENSION
	}

	panic("unreachable")
}

func criteriaTypeToModel(t gen.WorkflowCriteriaType) match.Type {
	switch t {
	case gen.EQUALS:
		return match.EQUALS
	case gen.NOTEQUALS:
		return match.NOT_EQUALS
	case gen.MATCHES:
		return match.MATCHES
	case gen.DOESNOTMATCH:
		return match.DOES_NOT_MATCH
	case gen.LESSTHAN:
		return match.LESS_THAN
	case gen.GREATERTHAN:
		return match.GREATER_THAN
	case gen.ISPRESENT:
		return match.IS_PRESENT
	case gen.ISNOTPRESENT:
		return match.IS_NOT_PRESENT
	}

	panic("unreachable")
}

func criteriaToModel(dto gen.WorkflowCriteria) match.Criteria {
	return match.Criteria{
		ID:          uuid.New(),
		Key:         criteriaKeyToModel(dto.Key),
		Type:        criteriaTypeToModel(dto.Type),
		Value:       dto.Value,
		CombineType: criteriaCombineTypeToModel(dto.CombineType),
	}
}

func getTargetId(target *ffmpeg.Target) uuid.UUID { return target.ID }
