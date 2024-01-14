package permissions

const (
	AccessIngestsPermission          string = "ingest:access"
	ResolveTroubledIngestsPermission string = "ingest:modify"
	DeleteIngestsPermission          string = "ingest:delete"
	PollNewIngestsPermission         string = "ingest:poll"

	AccessMediaPermission           string = "media:access"
	DeleteMediaPermission           string = "media:delete"
	StreamTranscodedMediaPermission string = "media:stream.pre"
	StreamSourceMediaPermission     string = "media:stream.source"
	StreamOnTheFlyMediaPermission   string = "media:stream.otf"

	CreateTranscodePermission string = "transcode:create"
	AccessTranscodePermission string = "transcode:access"
	ModifyTranscodePermission string = "transcode:modify"
	DeleteTranscodePermission string = "transcode:delete"

	CreateTargetPermission string = "target:create"
	AccessTargetPermission string = "target:access"
	EditTargetPermission   string = "target:modify"
	DeleteTargetPermission string = "target:delete"

	CreateWorkflowPermission string = "workflow:create"
	AccessWorkflowPermission string = "workflow:access"
	EditWorkflowPermission   string = "workflow:modify"
	DeleteWorkflowPermission string = "workflow:delete"

	CreateUserPermission          string = "user:create"
	AccessUserPermission          string = "user:access"
	EditUserPermissionsPermission string = "user:modify"
	DeleteUserPermission          string = "user:delete"
)

func All() []string {
	return []string{
		AccessIngestsPermission,
		ResolveTroubledIngestsPermission,
		DeleteIngestsPermission,
		PollNewIngestsPermission,
		AccessMediaPermission,
		DeleteMediaPermission,
		StreamTranscodedMediaPermission,
		StreamSourceMediaPermission,
		StreamOnTheFlyMediaPermission,
		CreateTranscodePermission,
		AccessTranscodePermission,
		ModifyTranscodePermission,
		DeleteTranscodePermission,
		CreateTargetPermission,
		AccessTargetPermission,
		EditTargetPermission,
		DeleteTargetPermission,
		CreateWorkflowPermission,
		AccessWorkflowPermission,
		EditWorkflowPermission,
		DeleteWorkflowPermission,
		CreateUserPermission,
		AccessUserPermission,
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
