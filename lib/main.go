package beaver

var (
	version   = ""
	commitSha = ""
	buildDate = ""
)

func Version() string {
	return version
}

func CommitSha() string {
	return commitSha
}

func BuildDate() string {
	return buildDate
}
