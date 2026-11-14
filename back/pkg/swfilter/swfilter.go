package swfilter

import "regexp"

type Reason string

const (
	ReasonUpdate    Reason = "SYSTEM_UPDATE"
	ReasonComponent Reason = "SYSTEM_COMPONENT"
)

var (
	updates    = regexp.MustCompile(`(?i)\bKB\d{6,8}\b|^(security update|update|hotfix) for `)
	components = regexp.MustCompile(`(?i)` +
		`visual c\+\+ .*(redistributable|runtime|minimum|additional)|redistributable runtime dlls|` +
		`^microsoft \.net (framework|runtime|desktop runtime|host)|^microsoft asp\.net core|` +
		`language pack|языковой пакет|proofing|правописани|korrekturhilfen|перевірки правопису|` +
		`\bshared mui\b|\(mui\)|\bmui\b \(|` +
		`\bdriver\b|драйвер|chipset|management engine|rapid storage|processor graphics|realtek|` +
		`^intel\(r\) (serial io|graphics|wireless|bluetooth|network)|^nvidia (graphics|hd audio|physx|frameview)|` +
		`windows sdk addon|vs_[a-z]|^vcpp_crt|^msxml|dhtml editing component`)
)

func Classify(name string) (Reason, bool) {
	switch {
	case updates.MatchString(name):
		return ReasonUpdate, true
	case components.MatchString(name):
		return ReasonComponent, true
	}
	return "", false
}

func Excluded(name string) bool {
	_, ok := Classify(name)
	return ok
}
