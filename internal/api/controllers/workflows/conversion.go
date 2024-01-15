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
		TargetIds: util.ApplyConversion(model.Targets, getTargetID),
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
	case match.TitleKey:
		return gen.TITLE
	case match.ResolutionKey:
		return gen.RESOLUTION
	case match.SeasonNumberKey:
		return gen.SEASONNUMBER
	case match.EpisodeNumberKey:
		return gen.EPISODENUMBER
	case match.SourcePathKey:
		return gen.SOURCEPATH
	case match.SourceNameKey:
		return gen.SOURCENAME
	case match.SourceExtensionKey:
		return gen.SOURCEEXTENSION
	}

	panic("unreachable")
}

func criteriaTypeToDto(t match.Type) gen.WorkflowCriteriaType {
	switch t {
	case match.Equals:
		return gen.EQUALS
	case match.NotEquals:
		return gen.NOTEQUALS
	case match.Matches:
		return gen.MATCHES
	case match.DoesNotMatch:
		return gen.DOESNOTMATCH
	case match.LessThan:
		return gen.LESSTHAN
	case match.GreaterThan:
		return gen.GREATERTHAN
	case match.IsPresent:
		return gen.ISPRESENT
	case match.IsNotPresent:
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
		return match.TitleKey
	case gen.RESOLUTION:
		return match.ResolutionKey
	case gen.SEASONNUMBER:
		return match.SeasonNumberKey
	case gen.EPISODENUMBER:
		return match.EpisodeNumberKey
	case gen.SOURCEPATH:
		return match.SourcePathKey
	case gen.SOURCENAME:
		return match.SourceNameKey
	case gen.SOURCEEXTENSION:
		return match.SourceExtensionKey
	}

	panic("unreachable")
}

func criteriaTypeToModel(t gen.WorkflowCriteriaType) match.Type {
	switch t {
	case gen.EQUALS:
		return match.Equals
	case gen.NOTEQUALS:
		return match.NotEquals
	case gen.MATCHES:
		return match.Matches
	case gen.DOESNOTMATCH:
		return match.DoesNotMatch
	case gen.LESSTHAN:
		return match.LessThan
	case gen.GREATERTHAN:
		return match.GreaterThan
	case gen.ISPRESENT:
		return match.IsPresent
	case gen.ISNOTPRESENT:
		return match.IsNotPresent
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

func getTargetID(target *ffmpeg.Target) uuid.UUID { return target.ID }
