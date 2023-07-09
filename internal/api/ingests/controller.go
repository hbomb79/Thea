package ingests

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/labstack/echo/v4"
)

type (
	// Dto is the response used by endpoints that return
	// the items being ingested (e.g., list, get)
	Dto struct {
		Id       uuid.UUID
		Path     string
		State    ingest.IngestItemState
		Trouble  *ingest.IngestItemTrouble
		Metadata *media.FileMediaMetadata
	}

	// Service is where this controller gets it's information from, this is
	// typically the Ingest service.
	Service interface {
		GetAllIngests() []*ingest.IngestItem
		GetIngest(uuid.UUID) *ingest.IngestItem
		RemoveIngest(uuid.UUID) error
	}

	// Controller is the struct which is responsible for defining the
	// routes for this controller. Additionally, it holds the reference to
	// the store used to retrieve information about ingests from Thea
	Controller struct {
		Service Service
	}
)

func New(serv Service) *Controller {
	return &Controller{Service: serv}
}

// Init accepts the Echo group for the ingest endpoints
// and sets the routes on them.
func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.GET("/", controller.list)
	eg.GET("/:id/", controller.get)
	eg.DELETE("/:id/", controller.delete)
	eg.POST("/:id/trouble-resolution/", controller.postTroubleResolution)
}

// list returns all the ingests - represented as DTOs - from the underlying store.
func (controller *Controller) list(ctx echo.Context) error {
	items := controller.Service.GetAllIngests()
	dtos := make([]*Dto, len(items))
	for k, v := range items {
		dtos[k] = NewDto(v)
	}

	return ctx.JSON(http.StatusOK, dtos)
}

// get uses the 'id' path param from the context and retrieves the ingest from the
// underlying store. If found, a DTO representing the ingest is returned
func (controller *Controller) get(ctx echo.Context) error {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ingest ID is not a valid UUID")
	}

	item := controller.Service.GetIngest(id)
	if item == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	return ctx.JSON(http.StatusOK, NewDto(item))
}

// delete uses the 'id' path param from the context and retrieves the ingest from the
// underlying store. If found, the Ingest is cancelled.
func (controller *Controller) delete(ctx echo.Context) error {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ingest ID is not a valid UUID")
	}

	if err := controller.Service.RemoveIngest(id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.NoContent(http.StatusOK)
}

// postTroubleResolution uses the 'id' path param from the context and retrieves the ingest
// from the underlying store. If found, then an attempt to resolve the trouble will be made.
func (controller *Controller) postTroubleResolution(ctx echo.Context) error {
	id, err := uuid.Parse(ctx.Param("id"))
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

	return ctx.NoContent(http.StatusOK)
}

// NewDto creates a IngestDto using the IngestItem model.
func NewDto(item *ingest.IngestItem) *Dto {
	return &Dto{
		Id:       item.Id,
		Path:     item.Path,
		State:    item.State,
		Trouble:  nil,
		Metadata: nil,
	}
}
