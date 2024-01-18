package ingests

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/labstack/echo/v4"
)

type (
	IngestService interface {
		GetAllIngests() []*ingest.IngestItem
		GetIngest(ingestID uuid.UUID) *ingest.IngestItem
		RemoveIngest(ingestID uuid.UUID) error
		DiscoverNewFiles()
		ResolveTroubledIngest(itemID uuid.UUID, method ingest.ResolutionType, context map[string]string) error
	}

	// IngestsController is the struct which is responsible for defining the
	// routes for this controller. Additionally, it holds the reference to
	// the store used to retrieve information about ingests from Thea.
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
// underlying store. If found, a DTO representing the ingest is returned.
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
