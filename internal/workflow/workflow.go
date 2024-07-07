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
	// If the workflow is 'disabled', then it's not allowed to automatically
	// run on new media. Currently, a disabled workflow can still be run against
	// media manually.
	if !workflow.Enabled {
		return false
	}

	// Check that this item matches the conditions specified by the profile. If there
	// are no conditions then just default to true
	if len(workflow.Criteria) == 0 {
		return true
	}

	currentEval := true
	skipToNextBlock := false
	for i, condition := range workflow.Criteria {
		// If a previous block failed, keep going until we find the
		// next 'or' block
		if skipToNextBlock && condition.CombineType == match.AND {
			continue
		}
		skipToNextBlock = false

		isMatch, err := condition.IsMediaAcceptable(media)
		if err != nil {
			log.Emit(logger.ERROR, "media %v is not eligible for criteria %v: %v\n", media, condition, err)
		}

		if condition.CombineType == match.OR {
			if currentEval {
				// End of this block, if the current block
				// is satisfied, then we're done, no need to
				// test the following conditions
				break
			}
		} else if i < len(workflow.Criteria) {
			// This condition is part of an unfinished block of conditions ANDed together.
			// If this condition FAILED to match, then we can skip until the next OR condition (if any)
			currentEval = isMatch
			skipToNextBlock = !isMatch
		}
	}

	return currentEval
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
