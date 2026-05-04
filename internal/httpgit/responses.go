package httpgit

import (
	"net/http"

	"github.com/luisalicea7/gitta-git-service/internal/api"
	"github.com/luisalicea7/gitta-git-service/internal/httpx"
)

func writeAuthDenied(w http.ResponseWriter, r *http.Request, auth api.AuthResponse) {
	switch auth.Reason {
	case "unauthorized":
		http.Error(w, "internal authorization failed", http.StatusBadGateway)
	case "invalid_credentials":
		httpx.WriteBasicAuthChallenge(w)
	case "insufficient_scope":
		http.Error(w, "forbidden", http.StatusForbidden)
	case "not_found":
		http.NotFound(w, r)
	default:
		http.Error(w, "authorization failed", http.StatusForbidden)
	}
}

func writePrepareFailure(w http.ResponseWriter, r *http.Request, result PrepareResult) {
	switch result {
	case PrepareNotFound:
		http.NotFound(w, r)
	default:
		http.Error(w, "repository preparation failed", http.StatusInternalServerError)
	}
}
