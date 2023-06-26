package controllers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/service/ingest"
	"github.com/labstack/echo/v4"
)

type (
	// IngestDto is the response used by endpoints that return
	// the items being ingested (e.g., list, get)
	IngestDto struct {
		id       uuid.UUID
		path     string
		state    ingest.IngestItemState
		trouble  *ingest.IngestItemTrouble
		metadata *media.FileMediaMetadata
	}

	// IngestStore is where this controller gets it's information from, this is
	// typically the Ingest service.
	IngestStore interface {
		AllItems() *[]*ingest.IngestItem
		Item(uuid.UUID) *ingest.IngestItem
		RemoveItem(uuid.UUID) error
	}

	// Ingests is the struct which is responsible for defining the
	// routes for this controller. Additionally, it holds the reference to
	// the store used to retrieve information about ingests from Thea
	Ingests struct {
		Store IngestStore
	}
)

// Init accepts the Echo group for the ingest endpoints
// and sets the routes on them.
func (controller *Ingests) SetRoutes(eg *echo.Group) {
	eg.GET("/", controller.list)
	eg.GET("/:id/", controller.get)
	eg.DELETE("/:id/", controller.delete)
	eg.POST("/:id/trouble-resolution/", controller.postTroubleResolution)
}

// list returns all the ingests - represented as DTOs - from the underlying store.
func (controller *Ingests) list(ctx echo.Context) error {
	items := controller.Store.AllItems()
	dtos := make([]*IngestDto, len(*items))
	for k, v := range *items {
		dtos[k] = newDto(v)
	}

	return ctx.JSON(http.StatusOK, dtos)
}

// get uses the 'id' path param from the context and retrieves the ingest from the
// underlying store. If found, a DTO representing the ingest is returned
func (controller *Ingests) get(ctx echo.Context) error {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ingest ID is not a valid UUID")
	}

	item := controller.Store.Item(id)
	if item == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	return ctx.JSON(http.StatusOK, newDto(item))
}

// delete uses the 'id' path param from the context and retrieves the ingest from the
// underlying store. If found, the Ingest is cancelled.
func (controller *Ingests) delete(ctx echo.Context) error {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ingest ID is not a valid UUID")
	}

	if err := controller.Store.RemoveItem(id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.NoContent(http.StatusOK)
}

// postTroubleResolution uses the 'id' path param from the context and retrieves the ingest
// from the underlying store. If found, then an attempt to resolve the trouble will be made.
func (controller *Ingests) postTroubleResolution(ctx echo.Context) error {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ingest ID is not a valid UUID")
	}

	item := controller.Store.Item(id)
	if item == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	if err := item.ResolveTrouble(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.NoContent(http.StatusOK)
}

// newDto creates a IngestDto using the IngestItem model.
func newDto(item *ingest.IngestItem) *IngestDto {
	return &IngestDto{
		id:       item.Id,
		path:     item.Path,
		state:    item.State,
		trouble:  nil,
		metadata: nil,
	}
}
