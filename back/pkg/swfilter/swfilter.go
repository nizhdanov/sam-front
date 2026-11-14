package swfilter

import "regexp"

var updates = regexp.MustCompile(`(?i)\bKB\d{6,8}\b|^(security update|update|hotfix) for `)

func Excluded(name string) bool { return updates.MatchString(name) }
