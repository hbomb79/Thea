package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/jmoiron/sqlx"
)

type (
	workflowModel struct {
		ID       uuid.UUID
		Enabled  bool
		Label    string
		Criteria jsonColumn[[]match.Criteria]
		Targets  jsonColumn[[]*ffmpeg.Target]
	}

	workflowTargetAssoc struct {
		ID         uuid.UUID
		WorkflowID uuid.UUID `db:"workflow_id"`
		TargetID   uuid.UUID `db:"target_id"`
	}

	jsonColumn[T any] struct {
		val *T
	}

	Store struct{}
)

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(Workflow{}, match.Criteria{})
}

// Create transactionally creates the workflow row, and the accompanying
// criteria table and workflow_target join table rows as needed.
func (store *Store) Create(db *sqlx.DB, workflowID uuid.UUID, label string, enabled bool, targetIDs []uuid.UUID, criteria []match.Criteria) error {
	fail := func(desc string, err error) error {
		return fmt.Errorf("failed to %s due to error: %s", desc, err.Error())
	}

	return database.WrapTx(db, func(tx *sqlx.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO workflow(id, created_at, updated_at, enabled, label)
			VALUES ($1, current_timestamp, current_timestamp, $2, $3)`,
			workflowID, label, enabled); err != nil {
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

func (store *Store) UpdateWorkflow(tx *sqlx.Tx, workflowID uuid.UUID, newLabel *string, newEnabled *bool) error {
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
		WHERE id=$1
		SET (updated_at, label, enabled) = (current_timestamp, $2, $3)
	`, workflowID, labelToSet, enabledToSet)

	return err
}

func (store *Store) UpdateWorkflowCriteria(tx *sqlx.Tx, workflowID uuid.UUID, criteria []match.Criteria) error {
	criteriaIDs := make([]uuid.UUID, len(criteria))
	for i, v := range criteria {
		criteriaIDs[i] = v.ID
	}

	// Insert workflow criteria, updating existing criteria
	if _, err := tx.NamedExec(`
		INSERT INTO workflow_criteria(id, created_at, updated_at, match_key, match_type, match_combine_type, match_value, workflow_id)
		VALUES(:id, current_timestamp, current_timestamp, :match_key, :match_type, :match_combine_type, :match_value, '`+workflowID.String()+`')
		ON CONFLICT DO UPDATE
			SET (updated_at, match_key, match_type, match_combine_type, match_value) =
				(current_timestamp, EXCLUDED.match_key, EXCLUDED.match_type, EXCLUDED.match_combine_type, EXCLUDED.match_value)
		`, criteria); err != nil {
		return err
	}

	// Drop workflow criteria rows which are no longer referenced
	// by this workflow
	if err := execDbIn(tx, `--sql
		DELETE FROM workflow_criteria wc
		WHERE wc.workflow_id='`+workflowID.String()+`'
			AND wc.id NOT IN (?)
		`, criteriaIDs); err != nil {
		return err
	}
	return nil

}
func (store *Store) UpdateWorkflowTargets(tx *sqlx.Tx, workflowID uuid.UUID, targetIDs []uuid.UUID) error {
	if _, err := tx.NamedExec(`DELETE FROM workflow_transcode_targets WHERE workflow_id=$1`, workflowID); err != nil {
		return err
	}

	_, err := tx.NamedExec(`
		INSERT INTO workflow_transcode_targets(id, workflow_id, transcode_target_id)
		VALUES(:id, :workflow_id, :target_id)
		`, buildWorkflowTargetAssocs(workflowID, targetIDs),
	)
	return err
}

func (store *Store) Get(db *sqlx.DB, id uuid.UUID) *Workflow {
	dest := &workflowModel{}
	if err := db.Get(dest, `
		SELECT w.*, JSONB_AGG(wc.*) AS criteria, JSONB_AGG(tt.*) AS targets
		FROM workflow w
		LEFT JOIN workflow_criteria wc
			ON wc.workflow_id = w.id
		LEFT JOIN workflow_transcode_targets wtt
			ON wtt.workflow_id = w.id
		LEFT JOIN transcode_targets tt
			ON tt.id = wtt.transcode_target_id
		WHERE w.id=$1
		GROUP BY w.id
	`, id); err != nil {
		log.Warnf("Failed to find workflow (id=%s): %s\n", id, err.Error())
		return nil
	}

	return &Workflow{dest.ID, dest.Enabled, dest.Label, *dest.Criteria.Get(), *dest.Targets.Get()}
}

func (store *Store) GetAll(db *sqlx.DB) []*Workflow {
	var dest []*workflowModel
	if err := db.Select(dest, `
		SELECT w.*, JSONB_AGG(wc.*) AS criteria, JSONB_AGG(tt.*) AS targets
		FROM workflow w
		LEFT JOIN workflow_criteria wc
			ON wc.workflow_id = w.id
		LEFT JOIN workflow_transcode_targets wtt
			ON wtt.workflow_id = w.id
		LEFT JOIN transcode_targets tt
			ON tt.id = wtt.transcode_target_id
		GROUP BY w.id
	`); err != nil {
		log.Warnf("Failed to get all workflows: %s\n", err.Error())
		return nil
	}

	output := make([]*Workflow, len(dest))
	for i, v := range dest {
		output[i] = &Workflow{v.ID, v.Enabled, v.Label, *v.Criteria.Get(), *v.Targets.Get()}
	}
	return output
}

func (store *Store) Delete(db *sqlx.DB, id uuid.UUID) {
	_, err := db.Exec(`DELETE FROM workflow WHERE id=$1;`, id)

	if err != nil {
		log.Fatalf("Failed to delete workflow with ID = %v due to error: %s\n", id, err.Error())
	}
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

func execDbIn(db *sqlx.Tx, query string, arg any) error {
	if q, a, e := sqlx.In(query, arg); e == nil {
		if _, err := db.Exec(db.Rebind(q), a); err != nil {
			return err
		}
	} else {
		return e
	}

	return nil
}
