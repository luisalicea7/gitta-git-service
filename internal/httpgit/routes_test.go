package httpgit

import "testing"

func TestParseRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want Route
	}{
		{
			name: "info refs",
			path: "/luis/my-repo.git/info/refs",
			want: Route{Owner: "luis", Repo: "my-repo", IsInfoRef: true},
		},
		{
			name: "upload pack rpc",
			path: "/luis/my-repo.git/git-upload-pack",
			want: Route{
				Owner:   "luis",
				Repo:    "my-repo",
				Service: "git-upload-pack",
				IsRPC:   true,
			},
		},
		{
			name: "receive pack rpc",
			path: "/luis/my-repo.git/git-receive-pack",
			want: Route{
				Owner:   "luis",
				Repo:    "my-repo",
				Service: "git-receive-pack",
				IsRPC:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseRoute(tt.path)
			if err != nil {
				t.Fatalf("ParseRoute() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseRoute() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseRouteRejectsInvalidPaths(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/",
		"/luis/my-repo/info/refs",
		"/luis/.git/info/refs",
		"/luis/my-repo.git/git-unknown",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseRoute(path); err == nil {
				t.Fatal("ParseRoute() error = nil, want error")
			}
		})
	}
}
