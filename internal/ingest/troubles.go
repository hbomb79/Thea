package ingest

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hbomb79/Thea/internal/http/tmdb"
)

type (
	TroubleType int
	Trouble     struct {
		error
		tType TroubleType

		// choices is a nullable list of search results; only populated
		// if the trouble type is TMDB_FAILURE_MULTI
		choices *[]tmdb.SearchResultItem
	}

	ResolutionType   int
	RetryResolution  struct{}
	AbortResolution  struct{}
	TmdbIDResolution struct{ tmdbID string }
)

const (
	MetadataFailure TroubleType = iota
	TmdbFailureUnknown
	TmdbFailureMultipleResults
	TmdbFailureNoResults
	UnknownFailure
)

const (
	Retry ResolutionType = iota
	SpecifyTmdbID
	Abort
)

var allowedResolutionTypes = map[TroubleType][]ResolutionType{
	MetadataFailure:            {Abort, Retry},
	UnknownFailure:             {Abort, Retry},
	TmdbFailureUnknown:         {Abort, Retry, SpecifyTmdbID},
	TmdbFailureMultipleResults: {Abort, Retry, SpecifyTmdbID},
	TmdbFailureNoResults:       {Abort, Retry, SpecifyTmdbID},
}

func newTrouble(err error) Trouble {
	var noResultError tmdb.NoResultError
	if errors.As(err, &noResultError) {
		return Trouble{error: err, tType: TmdbFailureNoResults}
	}

	var multipleResultError tmdb.MultipleResultError
	if errors.As(err, &multipleResultError) {
		return Trouble{error: err, tType: TmdbFailureMultipleResults, choices: multipleResultError.Choices()}
	}

	var illegalRequestError tmdb.IllegalRequestError
	if errors.As(err, &illegalRequestError) {
		return Trouble{error: err, tType: TmdbFailureUnknown}
	}

	return Trouble{error: err, tType: UnknownFailure}
}

func (t *Trouble) Type() TroubleType { return t.tType }

func (t *Trouble) AllowedResolutionTypes() []ResolutionType {
	if allowed, ok := allowedResolutionTypes[t.tType]; ok {
		return allowed
	}

	return []ResolutionType{}
}

func (t *Trouble) isResolutionTypeAllowed(resType ResolutionType) bool {
	for _, v := range t.AllowedResolutionTypes() {
		if v == resType {
			return true
		}
	}

	return false
}

func (t *Trouble) GenerateResolution(resolutionMethod ResolutionType, context map[string]string) (interface{}, error) {
	if !t.isResolutionTypeAllowed(resolutionMethod) {
		return nil, ErrResolutionIncompatible
	}

	switch resolutionMethod {
	case Abort:
		return &AbortResolution{}, nil
	case Retry:
		return &RetryResolution{}, nil
	case SpecifyTmdbID:
		if id, ok := context["tmdb_id"]; ok && len(strings.TrimSpace(id)) != 0 {
			return &TmdbIDResolution{tmdbID: id}, nil
		}

		return nil, ErrResolutionContextIncompatible
	default:
		return nil, ErrResolutionIncompatible
	}
}

// GetTmdbChoices returns the TMDB search result items for
// this trouble IF and ONLY IF the trouble type is TMDB_FAILURE_MULTI, and there
// are choices set on the trouble (non-nil). If either of these conditions
// are unmet, then `nil` is returned.
func (t *Trouble) GetTmdbChoices() []tmdb.SearchResultItem {
	if t.choices != nil && t.tType == TmdbFailureMultipleResults {
		return *t.choices
	}

	return nil
}

func (t TroubleType) String() string {
	//exhaustive:enforce
	switch t {
	case MetadataFailure:
		return fmt.Sprintf("METADATA_FAILURE[%d]", t)
	case TmdbFailureUnknown:
		return fmt.Sprintf("TMDB_FAILURE_UNKNOWN[%d]", t)
	case TmdbFailureMultipleResults:
		return fmt.Sprintf("TMDB_FAILURE_MULTI[%d]", t)
	case TmdbFailureNoResults:
		return fmt.Sprintf("TMDB_FAILURE_NONE[%d]", t)
	case UnknownFailure:
		return fmt.Sprintf("UNKNOWN_FAILURE[%d]", t)
	}

	panic("unreachable")
}
