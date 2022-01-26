package github

type RepositoryUrl string

type RepositoryRecord struct {
	ViewerPermission ViewerPermission `json:"viewerPermission"`
}

type ViewerPermission string

const (
	ViewerPermissionAdmin    ViewerPermission = "ADMIN"
	ViewerPermissionMaintain ViewerPermission = "MAINTAIN"
	ViewerPermissionWrite    ViewerPermission = "WRITE"
	ViewerPermissionTriage   ViewerPermission = "TRIAGE"
	ViewerPermissionRead     ViewerPermission = "READ"
)

type TokenState map[RepositoryUrl]RepositoryRecord
