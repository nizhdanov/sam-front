package prompts

import (
	"embed"
	"strings"

	"sam/pkg/auth"
)

const (
	MatchingVersion = "matching-v1"
	EULAVersion     = "eula-v2"
)

//go:embed chat/*.txt matching/*.txt eula/*.txt
var files embed.FS

func read(name string) string {
	body, err := files.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(body))
}

var (
	chatBase = read("chat/base.txt")
	byRole   = map[auth.Role]string{
		auth.RoleUser:  read("chat/user.txt"),
		auth.RoleAdmin: read("chat/admin.txt"),
	}
	matching = read("matching/software_matching.txt")
	eula     = read("eula/agreement_analysis.txt")
)

func Chat(role auth.Role) string {
	return chatBase + "\n\n" + byRole[role]
}

func Matching() string { return matching }

func EULA() string { return eula }
