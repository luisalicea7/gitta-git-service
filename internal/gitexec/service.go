package gitexec

type Service string

const (
	UploadPack  Service = "git-upload-pack"
	ReceivePack Service = "git-receive-pack"
)

func ServiceFromGitName(value string) (Service, bool) {
	switch value {
	case "git-upload-pack":
		return UploadPack, true
	case "git-receive-pack":
		return ReceivePack, true
	default:
		return "", false
	}
}

func (s Service) ShortName() string {
	switch s {
	case UploadPack:
		return "upload-pack"
	case ReceivePack:
		return "receive-pack"
	default:
		return ""
	}
}

func (s Service) AdvertisementContentType() string {
	return "application/x-" + string(s) + "-advertisement"
}

func (s Service) ResultContentType() string {
	return "application/x-" + string(s) + "-result"
}
