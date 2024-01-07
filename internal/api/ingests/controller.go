package ingests

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/labstack/echo/v4"
)

type (
	ResolutionTypeWrapper struct{ Value ingest.ResolutionType }
	ResolveTroubleRequest struct {
		Method  *ResolutionTypeWrapper `json:"method"`
		Context map[string]string      `json:"context"`
	}

	// IngestDto is the response used by endpoints that return
	// the items being ingested (e.g., list, get)
	IngestDto struct {
		Id       uuid.UUID                `json:"id"`
		Path     string                   `json:"source_path"`
		State    IngestStateDto           `json:"state"`
		Trouble  *TroubleDto              `json:"trouble"`
		Metadata *media.FileMediaMetadata `json:"file_metadata"`
	}

	IngestStateDto string
	TroubleTypeDto string

	TroubleDto struct {
		Type                   TroubleTypeDto          `json:"type"`
		Message                string                  `json:"message"`
		Context                map[string]any          `json:"context"`
		AllowedResolutionTypes []ResolutionTypeWrapper `json:"allowed_resolution_types"`
	}

	IngestService interface {
		GetAllIngests() []*ingest.IngestItem
		GetIngest(uuid.UUID) *ingest.IngestItem
		RemoveIngest(uuid.UUID) error
		DiscoverNewFiles()
		ResolveTroubledIngest(itemID uuid.UUID, method ingest.ResolutionType, context map[string]string) error
	}

	// Controller is the struct which is responsible for defining the
	// routes for this controller. Additionally, it holds the reference to
	// the store used to retrieve information about ingests from Thea
	Controller struct {
		service IngestService
	}
)

var controllerLogger = logger.Get("IngestsController")

const (
	IDLE        IngestStateDto = "IDLE"
	IMPORT_HOLD IngestStateDto = "IMPORT_HOLD"
	INGESTING   IngestStateDto = "INGESTING"
	TROUBLED    IngestStateDto = "TROUBLED"

	METADATA_FAILURE     TroubleTypeDto = "METADATA_FAILURE"
	TMDB_FAILURE_UNKNOWN TroubleTypeDto = "TMDB_FAILURE_UNKNOWN"
	TMDB_FAILURE_MULTI   TroubleTypeDto = "TMDB_FAILURE_MULTI_RESULT"
	TMDB_FAILURE_NONE    TroubleTypeDto = "TMDB_FAILURE_NO_RESULT"
	UNKNOWN_FAILURE      TroubleTypeDto = "UNKNOWN_FAILURE"
)

func New(validate *validator.Validate, serv IngestService) *Controller {
	return &Controller{service: serv}
}

// Init accepts the Echo group for the ingest endpoints
// and sets the routes on them.
func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.GET("/", controller.list)
	eg.POST("/poll/", controller.performPoll)
	eg.GET("/:id/", controller.get)
	eg.DELETE("/:id/", controller.delete)
	eg.POST("/:id/trouble-resolution/", controller.postTroubleResolution)
}

// list returns all the ingests - represented as DTOs - from the underlying store.
func (controller *Controller) list(ec echo.Context) error {
	items := controller.service.GetAllIngests()
	dtos := make([]*IngestDto, len(items))
	for k, v := range items {
		dtos[k] = NewDto(v)
	}

	return ec.JSON(http.StatusOK, dtos)
}

// get uses the 'id' path param from the context and retrieves the ingest from the
// underlying store. If found, a DTO representing the ingest is returned
func (controller *Controller) get(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ingest ID is not a valid UUID")
	}

	item := controller.service.GetIngest(id)
	if item == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	return ec.JSON(http.StatusOK, NewDto(item))
}

// delete uses the 'id' path param from the context and retrieves the ingest from the
// underlying store. If found, the Ingest is cancelled.
func (controller *Controller) delete(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ingest ID is not a valid UUID")
	}

	if err := controller.service.RemoveIngest(id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ec.NoContent(http.StatusOK)
}

// postTroubleResolution uses the 'id' path param from the context and retrieves the ingest
// from the underlying store. If found, then an attempt to resolve the trouble will be made.
func (controller *Controller) postTroubleResolution(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ingest ID is not a valid UUID")
	}

	var request ResolveTroubleRequest
	if err := ec.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("JSON body illegal: %v", err))
	} else if request.Method == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "JSON body missing mandatory 'method' field")
	}

	if err := controller.service.ResolveTroubledIngest(id, request.Method.Value, request.Context); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) performPoll(ec echo.Context) error {
	controller.service.DiscoverNewFiles()

	return ec.NoContent(http.StatusOK)
}

func (wrapper *ResolutionTypeWrapper) UnmarshalJSON(data []byte) error {
	var strValue string
	if err := json.Unmarshal(data, &strValue); err != nil {
		return err
	}

	switch strValue {
	case "abort":
		wrapper.Value = ingest.ABORT
	case "specify_tmdb_id":
		wrapper.Value = ingest.SPECIFY_TMDB_ID
	case "retry":
		wrapper.Value = ingest.RETRY
	default:
		return fmt.Errorf("invalid enum value: %s for resolution method", strValue)
	}

	return nil
}

func (wrapper *ResolutionTypeWrapper) MarshalJSON() ([]byte, error) {
	switch wrapper.Value {
	case ingest.ABORT:
		return json.Marshal("abort")
	case ingest.SPECIFY_TMDB_ID:
		return json.Marshal("specify_tmdb_id")
	case ingest.RETRY:
		return json.Marshal("retry")
	}

	return nil, fmt.Errorf("invalid enum value: %v for resolution method has no known marshalling", wrapper.Value)
}

// NewDto creates a IngestDto using the IngestItem model.
func NewDto(item *ingest.IngestItem) *IngestDto {
	var trbl *TroubleDto = nil
	if item.Trouble != nil {
		context, err := ExtractTroubleContext(item.Trouble)
		if err != nil {
			context = map[string]any{
				"_error": "Context for this trouble may be missing. Consult server logs for more information",
			}
			controllerLogger.Emit(logger.ERROR, "Error whilst creating DTO of ingestion trouble: %v\n", err)
		}

		trbl = &TroubleDto{
			Type:                   TroubleTypeModelToDto(item.Trouble.Type()),
			Message:                item.Trouble.Error(),
			Context:                context,
			AllowedResolutionTypes: ExtractTroubleResolutionTypes(item.Trouble),
		}
	}

	return &IngestDto{
		Id:       item.ID,
		Path:     item.Path,
		State:    IngestStateModelToDto(item.State),
		Trouble:  trbl,
		Metadata: item.ScrapedMetadata,
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

func ExtractTroubleResolutionTypes(trouble *ingest.Trouble) []ResolutionTypeWrapper {
	modelResTypes := trouble.AllowedResolutionTypes()
	dtoResTypes := make([]ResolutionTypeWrapper, len(modelResTypes))
	for k, v := range modelResTypes {
		dtoResTypes[k] = ResolutionTypeWrapper{Value: v}
	}

	return dtoResTypes
}

func TroubleTypeModelToDto(troubleType ingest.TroubleType) TroubleTypeDto {
	switch troubleType {
	case ingest.METADATA_FAILURE:
		return METADATA_FAILURE
	case ingest.TMDB_FAILURE_UNKNOWN:
		return TMDB_FAILURE_UNKNOWN
	case ingest.TMDB_FAILURE_NONE:
		return TMDB_FAILURE_NONE
	case ingest.TMDB_FAILURE_MULTI:
		return TMDB_FAILURE_MULTI
	case ingest.UNKNOWN_FAILURE:
		return UNKNOWN_FAILURE
	}

	panic(fmt.Sprintf("ingest trouble type %s is not recognized by API layer, DTO cannot be created. Please report this error.", troubleType))
}

func IngestStateModelToDto(modelType ingest.IngestItemState) IngestStateDto {
	switch modelType {
	case ingest.IDLE:
		return IDLE
	case ingest.IMPORT_HOLD:
		return IMPORT_HOLD
	case ingest.INGESTING:
		return INGESTING
	case ingest.TROUBLED:
		return TROUBLED
	}

	panic(fmt.Sprintf("ingest type %s is not recognized by API layer, DTO cannot be created. Please report this error.", modelType))
}
