package workflow

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Workflow")

type Workflow struct {
	ID       uuid.UUID
	Enabled  bool
	Label    string // unique
	Criteria []match.Criteria
	Targets  []*ffmpeg.Target // join table
}

func (workflow *Workflow) IsMediaEligible(media *media.Container) bool {
	// Check that this item matches the conditions specified by the profile. If there
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
			log.Emit(logger.ERROR, "media %v is not eligible for workflow %v: %v\n", media, workflow, err)
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

func (workflow *Workflow) SetCriteria(criteria []match.Criteria) error {
	for _, cond := range criteria {
		if err := cond.ValidateLegal(); err != nil {
			return err
		}
	}

	workflow.Criteria = criteria
	return nil
}
