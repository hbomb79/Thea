package ingests

import (
	"encoding/json"
	"fmt"

	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/api/util"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
)

type TmdbChoiceDTO struct {
	TmdbID     json.Number `json:"tmdb_id"`
	Adult      bool        `json:"is_adult"`
	Title      string      `json:"name"`
	Plot       string      `json:"overview"`
	PosterPath string      `json:"poster_url_path"`
}

func troubleResolutionDtoMethodToModel(method gen.IngestTroubleResolutionType) ingest.ResolutionType {
	switch method {
	case gen.ABORT:
		return ingest.Abort
	case gen.SPECIFYTMDBID:
		return ingest.SpecifyTmdbID
	case gen.RETRY:
		return ingest.Retry
	default:
		panic("invalid enum value for resolution method")
	}
}

func troubleResolutionModelMethodToDto(model ingest.ResolutionType) gen.IngestTroubleResolutionType {
	switch model {
	case ingest.Abort:
		return gen.ABORT
	case ingest.SpecifyTmdbID:
		return gen.SPECIFYTMDBID
	case ingest.Retry:
		return gen.RETRY
	}

	panic("invalid resolution type")
}

// NewDto creates a IngestDto using the IngestItem model.
func NewDto(item *ingest.IngestItem) gen.Ingest {
	var trbl *gen.IngestTrouble = nil
	if item.Trouble != nil {
		context, err := ExtractTroubleContext(item.Trouble)
		if err != nil {
			context = map[string]any{
				"_error": "Context for this trouble may be missing. Consult server logs for more information",
			}
			controllerLogger.Emit(logger.ERROR, "Error whilst creating DTO of ingestion trouble: %v\n", err)
		}

		trbl = &gen.IngestTrouble{
			Type:                   TroubleTypeModelToDto(item.Trouble.Type()),
			Message:                item.Trouble.Error(),
			Context:                context,
			AllowedResolutionTypes: ExtractTroubleResolutionTypes(item.Trouble),
		}
	}

	return gen.Ingest{
		Id:       item.ID,
		Path:     item.Path,
		State:    IngestStateModelToDto(item.State),
		Trouble:  trbl,
		Metadata: scrapedMetadataToDto(item.ScrapedMetadata),
	}
}

func ExtractTroubleContext(trouble *ingest.Trouble) (map[string]any, error) {
	switch trouble.Type() {
	case ingest.TmdbFailureMultipleResults:
		// Return a context which contains the choices we could make. The client will be expected
		// to use the unique TMDB ID of the choice when resolving this trouble.
		modelChoices := trouble.GetTmdbChoices()
		if modelChoices == nil {
			return nil, fmt.Errorf("failed to extract trouble context for %s. Type mandates presence of context which is not present, resulting trouble context will be missing expected information", trouble)
		}
		dtoChoices := make([]TmdbChoiceDTO, 0)
		for _, v := range trouble.GetTmdbChoices() {
			dtoChoices = append(dtoChoices, TmdbChoiceDTO{TmdbID: v.ID, Adult: v.Adult, Title: v.Title, Plot: v.Plot, PosterPath: v.PosterPath})
		}

		context := map[string]any{"choices": dtoChoices}
		return context, nil
	default:
		// Only multi-choice TMDB errors have context, all other ingestion errors are (at the moment)
		// context-free (i.e. the message and allowed actions alone should suffice).
		return map[string]any{}, nil
	}
}

func ExtractTroubleResolutionTypes(trouble *ingest.Trouble) []gen.IngestTroubleResolutionType {
	return util.ApplyConversion(trouble.AllowedResolutionTypes(), troubleResolutionModelMethodToDto)
}

func scrapedMetadataToDto(metadata *media.FileMediaMetadata) *gen.FileMetadata {
	if metadata == nil {
		return nil
	}

	return &gen.FileMetadata{
		EpisodeNumber: metadata.EpisodeNumber,
		Episodic:      metadata.Episodic,
		FrameHeight:   metadata.FrameH,
		FrameWidth:    metadata.FrameW,
		Path:          metadata.Path,
		Runtime:       metadata.Runtime,
		SeasonNumber:  metadata.SeasonNumber,
		Title:         metadata.Title,
		Year:          metadata.Year,
	}
}

func TroubleTypeModelToDto(troubleType ingest.TroubleType) gen.IngestTroubleType {
	switch troubleType {
	case ingest.MetadataFailure:
		return gen.METADATAFAILURE
	case ingest.TmdbFailureUnknown:
		return gen.TMDBFAILUREUNKNOWN
	case ingest.TmdbFailureNoResults:
		return gen.TMDBFAILURENORESULT
	case ingest.TmdbFailureMultipleResults:
		return gen.TMDBFAILUREMULTIRESULT
	case ingest.UnknownFailure:
		return gen.UNKNOWNFAILURE
	}

	panic(fmt.Sprintf("ingest trouble type %s is not recognized by API layer, DTO cannot be created. Please report this error.", troubleType))
}

func IngestStateModelToDto(modelType ingest.IngestItemState) gen.IngestState {
	switch modelType {
	case ingest.Idle:
		return gen.IngestStateIDLE
	case ingest.ImportHold:
		return gen.IngestStateIMPORTHOLD
	case ingest.Ingesting:
		return gen.IngestStateINGESTING
	case ingest.Troubled:
		return gen.IngestStateTROUBLED
	}

	panic(fmt.Sprintf("ingest type %s is not recognized by API layer, DTO cannot be created. Please report this error.", modelType))
}
