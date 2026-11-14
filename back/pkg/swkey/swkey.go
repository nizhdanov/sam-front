package swkey

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	spaces         = regexp.MustCompile(`\s+`)
	separators     = regexp.MustCompile(`[_|/\\]+`)
	trademarks     = strings.NewReplacer("®", "", "™", "", "©", "", "(r)", "", "(tm)", "", "(c)", "")
	publisherNoise = regexp.MustCompile(`[,.]?\s+(corporation|corp|incorporated|inc|llc|ltd|limited|gmbh|co|company|s\.?a\.?|ag|plc)\.?$`)
)

func Normalize(s string) string {
	s = norm.NFKC.String(s)
	s = strings.ToLower(s)
	s = trademarks.Replace(s)
	s = separators.ReplaceAllString(s, " ")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = spaces.ReplaceAllString(s, " ")
	return strings.Trim(s, " -.,")
}

func NormalizePublisher(s string) string {
	s = Normalize(s)
	for {
		trimmed := publisherNoise.ReplaceAllString(s, "")
		if trimmed == s {
			return strings.TrimSpace(s)
		}
		s = strings.TrimSpace(trimmed)
	}
}

func NormalizeVersion(s string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "v")
}

func SourceKey(name, publisher, version string) string {
	h := sha256.New()
	for _, field := range []string{Normalize(name), NormalizePublisher(publisher), NormalizeVersion(version)} {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(field)))
		h.Write(size[:])
		h.Write([]byte(field))
	}
	return hex.EncodeToString(h.Sum(nil))
}
