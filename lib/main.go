package beaver

var (
	version   = ""
	commitSha = ""
	buildDate = ""
)

func GetVersion() string {
	return version
}

func GetCommitSha() string {
	return commitSha
}

func GetBuildDate() string {
	return buildDate
}
