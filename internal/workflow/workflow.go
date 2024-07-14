package workflow

import (
	"fmt"
	"strings"

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

	currentEval := false
	skipToNextBlock := false

	// Builds a string during evaluation
	// of the conditions which reads like
	// false && SKIP || false || true && true ;
	debugStr := &strings.Builder{}
	defer func() {
		log.Emit(logger.VERBOSE, "Workflow %s condition evaluation for media %s debug string: "+debugStr.String(), workflow, media)
	}()

	for i, condition := range workflow.Criteria {
		fmt.Fprintf(debugStr, "(%d)", i)
		// If a previous block failed, keep going until we find the
		// next 'or' block
		if skipToNextBlock && condition.CombineType == match.AND {
			fmt.Fprintf(debugStr, "SKIP && ")
			continue
		}
		skipToNextBlock = false

		isMatch, err := condition.IsMediaAcceptable(media)
		if err != nil {
			log.Emit(logger.ERROR, "media %v is not eligible for criteria %v: %v\n", media, condition, err)
		}

		fmt.Fprintf(debugStr, "%v", isMatch)
		if condition.CombineType == match.OR {
			if currentEval {
				// End of this block, if the current block
				// is satisfied, then we're done, no need to
				// test the following conditions
				break
			}

			fmt.Fprintf(debugStr, " || ")
		} else {
			// This condition is part of an unfinished block of conditions ANDed together.
			// If this condition FAILED to match, then we can skip until the next OR condition (if any)
			currentEval = isMatch
			skipToNextBlock = !isMatch

			fmt.Fprintf(debugStr, " && ")
		}
	}

	fmt.Fprintf(debugStr, " -- DONE: %v;", currentEval)
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
