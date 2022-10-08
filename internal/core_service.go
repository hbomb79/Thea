package internal

type GetTroubleDetailsRequest struct{}
type ResolveTroubleRequest struct{}

type CoreService interface {
	GetTroubleDetails()
	ResolveTrouble()
}

func (coreApi *coreService) GetTroubleDetails() {

}

func (coreApi *coreService) ResolveTrouble() {

}

type coreService struct {
}

func NewCoreApi(tpa TPA) CoreService {
	return &coreService{}
}
