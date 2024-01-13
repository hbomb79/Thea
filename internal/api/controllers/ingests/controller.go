package ingests

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/labstack/echo/v4"
)

type (
	IngestService interface {
		GetAllIngests() []*ingest.IngestItem
		GetIngest(uuid.UUID) *ingest.IngestItem
		RemoveIngest(uuid.UUID) error
		DiscoverNewFiles()
		ResolveTroubledIngest(itemID uuid.UUID, method ingest.ResolutionType, context map[string]string) error
	}

	// IngestsController is the struct which is responsible for defining the
	// routes for this controller. Additionally, it holds the reference to
	// the store used to retrieve information about ingests from Thea
	IngestsController struct {
		service IngestService
	}
)

var controllerLogger = logger.Get("IngestsController")

func New(serv IngestService) *IngestsController {
	return &IngestsController{service: serv}
}

// ListIngests returns all the ingests - represented as DTOs - from the underlying store.
func (controller *IngestsController) ListIngests(ec echo.Context, _ gen.ListIngestsRequestObject) (gen.ListIngestsResponseObject, error) {
	items := controller.service.GetAllIngests()
	dtos := make([]gen.Ingest, len(items))
	for k, v := range items {
		dtos[k] = NewDto(v)
	}

	return gen.ListIngests200JSONResponse(dtos), nil
}

// GetIngest uses the 'id' path param from the context and retrieves the ingest from the
// underlying store. If found, a DTO representing the ingest is returned
func (controller *IngestsController) GetIngest(ec echo.Context, request gen.GetIngestRequestObject) (gen.GetIngestResponseObject, error) {
	item := controller.service.GetIngest(request.Id)
	if item == nil {
		return nil, echo.ErrNotFound
	}

	return gen.GetIngest200JSONResponse(NewDto(item)), nil
}

// DeleteIngest uses the 'id' path param from the context and retrieves the ingest from the
// underlying store. If found, the Ingest is cancelled.
func (controller *IngestsController) DeleteIngest(ec echo.Context, request gen.DeleteIngestRequestObject) (gen.DeleteIngestResponseObject, error) {
	if err := controller.service.RemoveIngest(request.Id); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return gen.DeleteIngest200Response{}, nil
}

// ResolveIngest uses the 'id' path param from the context and retrieves the ingest
// from the underlying store. If found, then an attempt to resolve the trouble will be made.
func (controller *IngestsController) ResolveIngest(ec echo.Context, request gen.ResolveIngestRequestObject) (gen.ResolveIngestResponseObject, error) {
	// TODO use validator for this
	if request.Body.Method == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "JSON body missing mandatory 'method' field")
	}

	if err := controller.service.ResolveTroubledIngest(
		request.Id,
		troubleResolutionDtoMethodToModel(request.Body.Method),
		request.Body.Context,
	); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return gen.ResolveIngest200Response{}, nil
}

func (controller *IngestsController) PollIngests(ec echo.Context, _ gen.PollIngestsRequestObject) (gen.PollIngestsResponseObject, error) {
	controller.service.DiscoverNewFiles()

	return gen.PollIngests200Response{}, nil
}

func troubleResolutionDtoMethodToModel(method gen.IngestTroubleResolutionType) ingest.ResolutionType {
	switch method {
	case "ABORT":
		return ingest.ABORT
	case "SPECIFY_TMDB_ID":
		return ingest.SPECIFY_TMDB_ID
	case "RETRY":
		return ingest.RETRY
	default:
		panic("invalid enum value for resolution method")
	}
}

func troubleResolutionModelMethodToDto(model ingest.ResolutionType) gen.IngestTroubleResolutionType {
	switch model {
	case ingest.ABORT:
		return "ABORT"
	case ingest.SPECIFY_TMDB_ID:
		return "SPECIFY_TMDB_ID"
	case ingest.RETRY:
		return "RETRY"
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

type TmdbChoiceDTO struct {
	TmdbId     json.Number `json:"tmdb_id"`
	Adult      bool        `json:"is_adult"`
	Title      string      `json:"name"`
	Plot       string      `json:"overview"`
	PosterPath string      `json:"poster_url_path"`
	// FirstAirDate *Date       `json:"first_air_date"`
	// ReleaseDate  *Date       `json:"release_date"`
}

func ExtractTroubleContext(trouble *ingest.Trouble) (map[string]any, error) {
	switch trouble.Type() {
	case ingest.TMDB_FAILURE_MULTI:
		// Return a context which contains the choices we could make. The client will be expected
		// to use the unique TMDB ID of the choice when resolving this trouble.
		modelChoices := trouble.GetTmdbChoices()
		if modelChoices == nil {
			return nil, fmt.Errorf("failed to extract trouble context for %s. Type mandates presence of context which is not present, resulting trouble context will be missing expected information", trouble)
		}
		dtoChoices := make([]TmdbChoiceDTO, 0)
		for _, v := range trouble.GetTmdbChoices() {
			dtoChoices = append(dtoChoices, TmdbChoiceDTO{TmdbId: v.Id, Adult: v.Adult, Title: v.Title, Plot: v.Plot, PosterPath: v.PosterPath})
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
	modelResTypes := trouble.AllowedResolutionTypes()
	dtoResTypes := make([]gen.IngestTroubleResolutionType, len(modelResTypes))
	for k, v := range modelResTypes {
		dtoResTypes[k] = troubleResolutionModelMethodToDto(v)
	}

	return dtoResTypes
}

func TroubleTypeModelToDto(troubleType ingest.TroubleType) gen.IngestTroubleType {
	switch troubleType {
	case ingest.METADATA_FAILURE:
		return gen.METADATAFAILURE
	case ingest.TMDB_FAILURE_UNKNOWN:
		return gen.TMDBFAILUREUNKNOWN
	case ingest.TMDB_FAILURE_NONE:
		return gen.TMDBFAILURENORESULT
	case ingest.TMDB_FAILURE_MULTI:
		return gen.TMDBFAILUREMULTIRESULT
	case ingest.UNKNOWN_FAILURE:
		return gen.UNKNOWNFAILURE
	}

	panic(fmt.Sprintf("ingest trouble type %s is not recognized by API layer, DTO cannot be created. Please report this error.", troubleType))
}

func IngestStateModelToDto(modelType ingest.IngestItemState) gen.IngestState {
	switch modelType {
	case ingest.IDLE:
		return gen.IngestStateIDLE
	case ingest.IMPORT_HOLD:
		return gen.IngestStateIMPORTHOLD
	case ingest.INGESTING:
		return gen.IngestStateINGESTING
	case ingest.TROUBLED:
		return gen.IngestStateTROUBLED
	}

	panic(fmt.Sprintf("ingest type %s is not recognized by API layer, DTO cannot be created. Please report this error.", modelType))
}
