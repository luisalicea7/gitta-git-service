package repos

import (
	"os"
	"path/filepath"
)

const preReceiveHook = `#!/bin/sh
set -eu

updates=""
while read old_sha new_sha ref_name; do
	if [ -z "$old_sha" ] || [ -z "$new_sha" ] || [ -z "$ref_name" ]; then
		continue
	fi
	entry="{\"oldSha\":\"$old_sha\",\"newSha\":\"$new_sha\",\"ref\":\"$ref_name\"}"
	if [ -z "$updates" ]; then
		updates="$entry"
	else
		updates="$updates,$entry"
	fi
done

payload="{\"repositoryId\":\"$GITTA_REPOSITORY_ID\",\"userId\":\"$GITTA_USER_ID\",\"permission\":\"$GITTA_PERMISSION\",\"updates\":[$updates]}"
response="$(curl -sS -w '\n%{http_code}' -X POST "$GITTA_API_URL/internal/git/pre-receive" -H "content-type: application/json" -H "x-gitta-internal-secret: $GITTA_INTERNAL_SECRET" --data "$payload" || true)"
status="$(printf '%s' "$response" | tail -n 1)"
body="$(printf '%s' "$response" | sed '$d')"

case "$status" in
	2*) exit 0 ;;
esac

if [ -n "$body" ]; then
	printf '%s\n' "$body" >&2
else
	printf '%s\n' "branch protection check failed" >&2
fi
exit 1
`

func EnsurePreReceiveHook(repoPath string) error {
	hooksDir := filepath.Join(repoPath, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(hooksDir, "pre-receive"), []byte(preReceiveHook), 0o750)
}
