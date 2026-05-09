package branches

type branchRequest struct {
	RepositoryID string `json:"repositoryId"`
	OwnerUserID  string `json:"ownerUserId"`
	Name         string `json:"name"`
	SHA          string `json:"sha"`
}
