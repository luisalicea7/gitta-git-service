package gitexec

import "testing"

func TestServiceFromGitName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  Service
		ok    bool
	}{
		{input: "git-upload-pack", want: UploadPack, ok: true},
		{input: "git-receive-pack", want: ReceivePack, ok: true},
		{input: "git-unknown", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got, ok := ServiceFromGitName(tt.input)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("ServiceFromGitName() = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
