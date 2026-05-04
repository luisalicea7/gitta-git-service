package httpgit

import (
	"errors"
	"strings"
)

type Route struct {
	Owner     string
	Repo      string
	Service   string
	IsInfoRef bool
	IsRPC     bool
}

func ParseRoute(path string) (Route, error) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 && len(parts) != 4 {
		return Route{}, errors.New("invalid git route")
	}

	if !strings.HasSuffix(parts[1], ".git") {
		return Route{}, errors.New("missing .git suffix")
	}

	route := Route{
		Owner: parts[0],
		Repo:  strings.TrimSuffix(parts[1], ".git"),
	}

	if route.Owner == "" || route.Repo == "" {
		return Route{}, errors.New("missing owner or repo")
	}

	if len(parts) == 4 && parts[2] == "info" && parts[3] == "refs" {
		route.IsInfoRef = true
		return route, nil
	}

	if len(parts) == 3 {
		switch parts[2] {
		case "git-upload-pack", "git-receive-pack":
			route.Service = parts[2]
			route.IsRPC = true
			return route, nil
		}
	}

	return Route{}, errors.New("unsupported git route")
}
