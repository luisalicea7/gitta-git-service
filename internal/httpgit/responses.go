package httpgit

import (
	"net/http"

	"github.com/luisalicea7/gitta-git-service/internal/api"
	"github.com/luisalicea7/gitta-git-service/internal/gitexec"
	"github.com/luisalicea7/gitta-git-service/internal/httpx"
)

func writeAuthDenied(w http.ResponseWriter, auth api.AuthResponse, service gitexec.Service) {
	switch auth.Reason {
	case "unauthorized":
		http.Error(w, "internal authorization failed", http.StatusBadGateway)
	case "invalid_credentials":
		httpx.WriteBasicAuthChallenge(w)
	case "insufficient_scope":
		http.Error(w, insufficientScopeMessage(service), http.StatusForbidden)
	case "not_found":
		http.Error(w, "repository not found", http.StatusNotFound)
	default:
		http.Error(w, "authorization failed", http.StatusForbidden)
	}
}

func insufficientScopeMessage(service gitexec.Service) string {
	if service == gitexec.ReceivePack {
		return "token does not have repo:write scope"
	}

	return "token does not have repo:read scope"
}

func writePrepareFailure(w http.ResponseWriter, result PrepareResult) {
	switch result {
	case PrepareNotFound:
		http.Error(w, "repository not found", http.StatusNotFound)
	default:
		http.Error(w, "repository preparation failed", http.StatusInternalServerError)
	}
}
