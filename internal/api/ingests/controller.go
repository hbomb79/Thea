package ingests

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/labstack/echo/v4"
)

type (
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
		Type    TroubleTypeDto `json:"type"`
		Message string         `json:"message"`
		Context map[string]any `json:"context"`
	}

	// Service is where this controller gets it's information from, this is
	// typically the Ingest service.
	Service interface {
		GetAllIngests() []*ingest.IngestItem
		GetIngest(uuid.UUID) *ingest.IngestItem
		RemoveIngest(uuid.UUID) error
		DiscoverNewFiles()
	}

	// Controller is the struct which is responsible for defining the
	// routes for this controller. Additionally, it holds the reference to
	// the store used to retrieve information about ingests from Thea
	Controller struct {
		Service Service
	}
)

const (
	IDLE        IngestStateDto = "IDLE"
	IMPORT_HOLD IngestStateDto = "IMPORT_HOLD"
	INGESTING   IngestStateDto = "INGESTING"
	TROUBLED    IngestStateDto = "TROUBLED"

	METADATA_FAILURE     TroubleTypeDto = "METADATA_FAILURE"
	TMDB_FAILURE_UNKNOWN TroubleTypeDto = "TMDB_FAILURE_UNKNOWN"
	TMDB_FAILURE_MULTI   TroubleTypeDto = "TMDB_FAILURE_MULTI_RESULT"
	TMDB_FAILURE_NONE    TroubleTypeDto = "TMDB_FAILURE_NO_RESULT"
)

func New(validate *validator.Validate, serv Service) *Controller {
	return &Controller{Service: serv}
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
	items := controller.Service.GetAllIngests()
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

	item := controller.Service.GetIngest(id)
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

	if err := controller.Service.RemoveIngest(id); err != nil {
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

	item := controller.Service.GetIngest(id)
	if item == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	if err := item.ResolveTrouble(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) performPoll(ec echo.Context) error {
	controller.Service.DiscoverNewFiles()

	return ec.NoContent(http.StatusOK)
}

// NewDto creates a IngestDto using the IngestItem model.
func NewDto(item *ingest.IngestItem) *IngestDto {
	var trbl *TroubleDto = nil
	if item.Trouble != nil {
		trbl = &TroubleDto{
			Type:    TroubleTypeModelToDto(item.Trouble.Type),
			Message: item.Trouble.Error(),
			Context: map[string]any{},
		}
	}

	return &IngestDto{
		Id:       item.Id,
		Path:     item.Path,
		State:    IngestStateModelToDto(item.State),
		Trouble:  trbl,
		Metadata: item.ScrapedMetadata,
	}
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
