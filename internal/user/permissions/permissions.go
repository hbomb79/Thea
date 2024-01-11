package permissions

const (
	ViewIngestsPermission            string = "ingest:view"
	ResolveTroubledIngestsPermission string = "ingest:modify"
	DeleteIngestsPermission          string = "ingest:delete"
	PollNewIngestsPermission         string = "ingest:poll"

	ViewMediaPermission             string = "media:view"
	DeleteMediaPermission           string = "media:delete"
	StreamTranscodedMediaPermission string = "media:stream.pre"
	StreamSourceMediaPermission     string = "media:stream.source"
	StreamOnTheFlyMediaPermission   string = "media:stream.otf"

	CreateTranscodePermission string = "transcode:new"
	ViewTranscodePermission   string = "transcode:view"
	ModifyTranscodePermission string = "transcode:modify"
	DeleteTranscodePermission string = "transcode:delete"

	CreateTargetPermission string = "target:new"
	ViewTargetPermission   string = "target:view"
	EditTargetPermission   string = "target:modify"
	DeleteTargetPermission string = "target:delete"

	CreateWorkflowPermission string = "workflow:new"
	ViewWorkflowPermission   string = "workflow:view"
	EditWorkflowPermission   string = "workflow:modify"
	DeleteWorkflowPermission string = "workflow:delete"

	CreateUserPermission          string = "user:new"
	ViewUserPermission            string = "user:view"
	EditUserPermissionsPermission string = "user:modify"
	DeleteUserPermission          string = "user:delete"
)

func All() []string {
	return []string{
		ViewIngestsPermission,
		ResolveTroubledIngestsPermission,
		DeleteIngestsPermission,
		PollNewIngestsPermission,
		ViewMediaPermission,
		DeleteMediaPermission,
		StreamTranscodedMediaPermission,
		StreamSourceMediaPermission,
		StreamOnTheFlyMediaPermission,
		CreateTranscodePermission,
		ViewTranscodePermission,
		ModifyTranscodePermission,
		DeleteTranscodePermission,
		CreateTargetPermission,
		ViewTargetPermission,
		EditTargetPermission,
		DeleteTargetPermission,
		CreateWorkflowPermission,
		ViewWorkflowPermission,
		EditWorkflowPermission,
		DeleteWorkflowPermission,
		CreateUserPermission,
		ViewUserPermission,
		EditUserPermissionsPermission,
		DeleteUserPermission,
	}
}

func Set() map[string]struct{} {
	all := All()
	set := make(map[string]struct{}, len(all))
	for _, v := range all {
		set[v] = struct{}{}
	}

	return set
}
