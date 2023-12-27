package workflow

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/jmoiron/sqlx"
)

type (
	workflowModel struct {
		ID        uuid.UUID                    `db:"id"`
		UpdatedAt time.Time                    `db:"updated_at"`
		CreatedAt time.Time                    `db:"created_at"`
		Enabled   bool                         `db:"enabled"`
		Label     string                       `db:"label"`
		Criteria  jsonColumn[[]match.Criteria] `db:"criteria"`
		Targets   jsonColumn[[]*ffmpeg.Target] `db:"targets"`
	}

	workflowTargetAssoc struct {
		ID         uuid.UUID `db:"id"`
		WorkflowID uuid.UUID `db:"workflow_id"`
		TargetID   uuid.UUID `db:"target_id"`
	}

	jsonColumn[T any] struct {
		val *T
	}

	Store struct{}
)

// Create transactionally creates the workflow row, and the accompanying
// criteria table and workflow_target join table rows as needed.
func (store *Store) Create(db *sqlx.DB, workflowID uuid.UUID, label string, enabled bool, targetIDs []uuid.UUID, criteria []match.Criteria) error {
	fail := func(desc string, err error) error {
		return fmt.Errorf("failed to %s: %w", desc, err)
	}

	return database.WrapTx(db, func(tx *sqlx.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO workflow(id, created_at, updated_at, enabled, label)
			VALUES ($1, current_timestamp, current_timestamp, $2, $3)`,
			workflowID, enabled, label); err != nil {
			return fail("create workflow row", err)
		}

		if _, err := tx.NamedExec(`
			INSERT INTO workflow_transcode_targets(id, workflow_id, transcode_target_id)
			VALUES(:id, :workflow_id, :target_id)`,
			buildWorkflowTargetAssocs(workflowID, targetIDs)); err != nil {
			return fail("create workflow target associations", err)
		}

		if _, err := tx.NamedExec(`
			INSERT INTO workflow_criteria(id, created_at, updated_at, match_key, match_type, match_value, match_combine_type, workflow_id)
			VALUES (:id, current_timestamp, current_timestamp, :match_key, :match_type, :match_value, :match_combine_type, '`+workflowID.String()+`')`,
			criteria); err != nil {
			return fail("create workflow criteria associations", err)
		}

		return nil
	})
}

// UpdateWorkflowTx updates only the workflows main data, such as it's label.
//
// NOTE: This action is intended to be used as part of an over-arching transaction; user-story
// for updating a workflow should consider all related data too.
func (store *Store) UpdateWorkflowTx(tx *sqlx.Tx, workflowID uuid.UUID, newLabel *string, newEnabled *bool) error {
	var labelToSet string
	var enabledToSet bool
	if err := tx.QueryRowx(`SELECT label, enabled FROM workflow WHERE id=$1`, workflowID).Scan(&labelToSet, &enabledToSet); err != nil {
		return err
	}

	if newLabel != nil {
		labelToSet = *newLabel
	}
	if newEnabled != nil {
		enabledToSet = *newEnabled
	}

	_, err := tx.Exec(`
		UPDATE workflow
		SET (updated_at, label, enabled) = (current_timestamp, $2, $3)
		WHERE id=$1
	`, workflowID, labelToSet, enabledToSet)

	return err
}

// UpdateWorkflowCriteriaTx updates only the workflows related match criteria. The criteria provided
// *replaces* the existing criteria. That is to say, criteria will be created, updated and deletes
// as needed.
//
// NOTE: This action is intended to be used as part of an over-arching transaction; user-story
// for updating a workflow should consider all related data too.
func (store *Store) UpdateWorkflowCriteriaTx(tx *sqlx.Tx, workflowID uuid.UUID, criteria []match.Criteria) error {
	criteriaIDs := make([]uuid.UUID, len(criteria))
	for i, v := range criteria {
		criteriaIDs[i] = v.ID
	}

	// Insert workflow criteria, updating existing criteria
	if len(criteria) > 0 {
		if _, err := tx.NamedExec(`
			INSERT INTO workflow_criteria(id, created_at, updated_at, match_key, match_type, match_combine_type, match_value, workflow_id)
			VALUES(:id, current_timestamp, current_timestamp, :match_key, :match_type, :match_combine_type, :match_value, '`+workflowID.String()+`')
			ON CONFLICT(id) DO UPDATE
				SET (updated_at, match_key, match_type, match_combine_type, match_value) =
					(current_timestamp, EXCLUDED.match_key, EXCLUDED.match_type, EXCLUDED.match_combine_type, EXCLUDED.match_value)
		`, criteria); err != nil {
			return err
		}

		// Drop workflow criteria rows which are no longer referenced
		// by this workflow
		if err := database.InExec(tx, `--sql
			DELETE FROM workflow_criteria wc
			WHERE wc.workflow_id='`+workflowID.String()+`'
				AND wc.id NOT IN (?)
			`, criteriaIDs); err != nil {
			return err
		}
	} else {
		_, err := tx.Exec(`--sql
		DELETE FROM workflow_criteria WHERE workflow_id='` + workflowID.String() + `'`)
		return err
	}

	return nil
}

// UpdateWorkflowTargetsTx updates a workflows transcode targets by modifying the rows
// in the join table as needed. For simplicitly, this function will drop all rows
// for the given workflow and re-create them.
//
// NOTE: This DB action is intended to be used as part of an over-arching transaction; user-story
// for updating a workflow should consider all related data too.
func (store *Store) UpdateWorkflowTargetsTx(tx *sqlx.Tx, workflowID uuid.UUID, targetIDs []uuid.UUID) error {
	if _, err := tx.Exec(`DELETE FROM workflow_transcode_targets WHERE workflow_id=$1`, workflowID); err != nil {
		return err
	}

	if len(targetIDs) > 0 {
		_, err := tx.NamedExec(`
			INSERT INTO workflow_transcode_targets(id, workflow_id, transcode_target_id)
			VALUES(:id, :workflow_id, :target_id)
			`, buildWorkflowTargetAssocs(workflowID, targetIDs),
		)

		return err
	}

	return nil
}

// Get queries the database for a specific workflow, and all it's related information.
// The workflows criteria/targets are accessed via a join and aggregated in to
// the result row as a JSONB array, which is then unmarshalled and used to
// construct a 'Workflow'
func (store *Store) Get(db *sqlx.DB, id uuid.UUID) *Workflow {
	dest := &workflowModel{}
	if err := db.Get(dest, getWorkflowSql(`WHERE w.id=$1`), id); err != nil {
		log.Warnf("Failed to find workflow (id=%s): %v\n", id, err)
		return nil
	}

	return &Workflow{dest.ID, dest.Enabled, dest.Label, *dest.Criteria.Get(), *dest.Targets.Get()}
}

// GetAll queries the database for all workflows, and all the related information.
// The workflows criteria/targets are accessed via a join and aggregated in to
// the result row as a JSONB array, which is then unmarshalled and used to
// construct a 'Workflow'
func (store *Store) GetAll(db *sqlx.DB) []*Workflow {
	var dest []*workflowModel
	if err := db.Select(&dest, getWorkflowSql("")); err != nil {
		log.Warnf("Failed to get all workflows: %v\n", err)
		return nil
	}

	output := make([]*Workflow, len(dest))
	for i, v := range dest {
		output[i] = &Workflow{v.ID, v.Enabled, v.Label, *v.Criteria.Get(), *v.Targets.Get()}
	}
	return output
}

// Delete will remove a workflow, and all it's related information (by way of cascading deletes)
// using the workflow ID provided. To delete only the workflows criteria/targets/etc,
// the relevant update method should be used instead.
func (store *Store) Delete(db *sqlx.DB, id uuid.UUID) {
	_, err := db.Exec(`DELETE FROM workflow WHERE id=$1;`, id)

	if err != nil {
		log.Fatalf("Failed to delete workflow with ID = %v due to error: %v\n", id, err)
	}
}

func getWorkflowSql(whereClause string) string {
	return fmt.Sprintf(`
		SELECT
			w.*,
			COALESCE(JSONB_AGG(DISTINCT wc.*) FILTER (WHERE wc.id IS NOT NULL), '[]') AS criteria,
			COALESCE(JSONB_AGG(DISTINCT tt.*) FILTER (WHERE tt.id IS NOT NULL), '[]') AS targets
		FROM workflow w
		LEFT JOIN workflow_criteria wc
			ON wc.workflow_id = w.id
		LEFT JOIN workflow_transcode_targets wtt
			ON wtt.workflow_id = w.id
		LEFT JOIN transcode_target tt
			ON tt.id = wtt.transcode_target_id
		%s
		GROUP BY w.id
	`, whereClause)
}

func (j *jsonColumn[T]) Scan(src any) error {
	if src == nil {
		j.val = nil
		return nil
	}

	j.val = new(T)
	return json.Unmarshal(src.([]byte), j.val)
}

func (j *jsonColumn[T]) Get() *T {
	return j.val
}

func buildWorkflowTargetAssocs(workflowID uuid.UUID, targetIDs []uuid.UUID) []workflowTargetAssoc {
	assocs := make([]workflowTargetAssoc, len(targetIDs))
	for i, v := range targetIDs {
		assocs[i] = workflowTargetAssoc{uuid.New(), workflowID, v}
	}

	return assocs
}
