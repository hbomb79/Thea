package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/jmoiron/sqlx"
)

var (
	WorkflowTable         = goqu.T("workflow")
	WorkflowCriteriaTable = goqu.T("workflow_criteria")
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

	tx, err := db.Beginx()
	if err != nil {
		return fail("open transaction", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO workflow(id, created_at, updated_at, enabled, label)
		VALUES ($1, current_timestamp, current_timestamp, $2, $3)
	`, workflowID, label, enabled); err != nil {
		return fail("create workflow row", err)
	}

	if _, err := tx.NamedExec(`
		INSERT INTO workflow_transcode_targets(id, workflow_id, transcode_target_id)
		VALUES(:id, :workflow_id, :target_id)
	`, BuildWorkflowTargetAssocs(workflowID, targetIDs)); err != nil {
		return fail("create workflow target associations", err)
	}

	if _, err := tx.NamedExec(`
		INSERT INTO workflow_criteria(id, created_at, updated_at, match_key, match_type, match_value, match_combine_type, workflow_id)
		VALUES (:id, current_timestamp, current_timestamp, :match_key, :match_type, :match_value, :match_combine_type, '`+workflowID.String()+`')
	`, criteria); err != nil {
		return fail("create workflow criteria associations", err)
	}

	if err := tx.Commit(); err != nil {
		return fail("commit workflow creation transaction", err)
	}

	return nil
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

func BuildWorkflowTargetAssocs(workflowID uuid.UUID, targetIDs []uuid.UUID) []workflowTargetAssoc {
	assocs := make([]workflowTargetAssoc, len(targetIDs))
	for i, v := range targetIDs {
		assocs[i] = workflowTargetAssoc{uuid.New(), workflowID, v}
	}

	return assocs
}
