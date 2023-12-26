package ingest

import (
	"fmt"

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
	METADATA_FAILURE TroubleType = iota
	TMDB_FAILURE_UNKNOWN
	TMDB_FAILURE_MULTI
	TMDB_FAILURE_NONE
	GENERIC_FAILURE

	RETRY ResolutionType = iota
	SPECIFY_TMDB_ID
	ABORT
)

var (
	allowedResolutionTypes = map[TroubleType][]ResolutionType{
		METADATA_FAILURE:     {ABORT, RETRY},
		TMDB_FAILURE_UNKNOWN: {ABORT, RETRY, SPECIFY_TMDB_ID},
		TMDB_FAILURE_MULTI:   {ABORT, RETRY, SPECIFY_TMDB_ID},
		TMDB_FAILURE_NONE:    {ABORT, RETRY, SPECIFY_TMDB_ID},
	}
)

func newTrouble(err error) Trouble {
	switch err := err.(type) {
	case *tmdb.NoResultError:
		return Trouble{error: err, tType: TMDB_FAILURE_NONE}
	case *tmdb.MultipleResultError:
		return Trouble{error: err, tType: TMDB_FAILURE_MULTI, choices: &err.Results}
	case *tmdb.IllegalRequestError:
		return Trouble{error: err, tType: TMDB_FAILURE_UNKNOWN}
	}

	return Trouble{error: err, tType: GENERIC_FAILURE}
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
	case ABORT:
		return &AbortResolution{}, nil
	case RETRY:
		return &RetryResolution{}, nil
	case SPECIFY_TMDB_ID:
		if id, ok := context["tmdb_id"]; ok {
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
	if t.choices != nil && t.tType == TMDB_FAILURE_MULTI {
		return *t.choices
	}

	return nil
}

func (t TroubleType) String() string {
	switch t {
	case METADATA_FAILURE:
		return fmt.Sprintf("METADATA_FAILURE[%d]", t)
	case TMDB_FAILURE_UNKNOWN:
		return fmt.Sprintf("TMDB_FAILURE_UNKNOWN[%d]", t)
	case TMDB_FAILURE_MULTI:
		return fmt.Sprintf("TMDB_FAILURE_MULTI[%d]", t)
	case TMDB_FAILURE_NONE:
		return fmt.Sprintf("TMDB_FAILURE_NONE[%d]", t)
	default:
		return fmt.Sprintf("UNKNOWN[%d]", t)
	}
}
