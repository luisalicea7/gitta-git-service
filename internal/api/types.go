package api

type GitOperation string

const (
	OperationUploadPack  GitOperation = "upload-pack"
	OperationReceivePack GitOperation = "receive-pack"
)

type AuthRequest struct {
	Username  string       `json:"username"`
	Token     string       `json:"token"`
	Owner     string       `json:"owner"`
	Repo      string       `json:"repo"`
	Operation GitOperation `json:"operation"`
}

type Repository struct {
	ID            string `json:"id"`
	OwnerUserID   string `json:"ownerUserId"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	DefaultBranch string `json:"defaultBranch"`
	StoragePath   string `json:"storagePath"`
}

type AuthResponse struct {
	Allowed    bool       `json:"allowed"`
	Reason     string     `json:"reason"`
	UserID     string     `json:"userId"`
	Repository Repository `json:"repository"`
	Permission string     `json:"permission"`
}

type GitRef struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
	Type string `json:"type"`
}

type PostReceiveRequest struct {
	RepositoryID string   `json:"repositoryId"`
	Refs         []GitRef `json:"refs"`
}

type PostReceiveResponse struct {
	Status string `json:"status"`
	Synced int    `json:"synced"`
	Reason string `json:"reason"`
}
