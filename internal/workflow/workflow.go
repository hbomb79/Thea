package workflow

import (
	"sync"

	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Workflow")

type Workflow struct {
	*sync.Mutex
	Label    string
	Criteria []*match.Criteria
	Targets  []*ffmpeg.Target
}

// Newprofile accepts a single string argument (tag) and returns a new profile
// be reference to the caller with it's internal targets and tag set.
func NewWorkflow() *Workflow {
	return nil
}

func (workflow *Workflow) IsMediaEligible(media *media.Container) bool {
	// Check that this item matches the the conditions specified by the profile. If there
	// are no conditions, we assume this profile has none and will return true
	if len(workflow.Criteria) == 0 {
		return true
	}

	currentEval := true
	skipToNextBlock := false
	for _, condition := range workflow.Criteria {
		// If a previous block failed, keep going until we find the
		// next 'or' block
		if skipToNextBlock && condition.CombineType == match.AND {
			continue
		}
		skipToNextBlock = false

		isMatch, err := condition.IsMediaAcceptable(media)
		if err != nil {
			log.Emit(logger.ERROR, "media %v is not eligible for workflow %v: %s\n", media, workflow, err.Error())
		}

		if !isMatch {
			skipToNextBlock = true
			currentEval = true

			continue
		}

		if currentEval {
			currentEval = isMatch
		}

		if condition.CombineType == match.OR {
			// End of this block
			if currentEval {
				return true
			} else {
				currentEval = true
			}
		}
	}

	return false
}

func (profile *Workflow) SetCriteria(criteria []*match.Criteria) error {
	for _, cond := range criteria {
		if err := cond.ValidateLegal(); err != nil {
			return err
		}
	}

	profile.Criteria = criteria
	return nil
}
