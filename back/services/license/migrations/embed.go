package migrations

import (
	"embed"
	"io/fs"
)

//go:embed postgres/*.sql
var postgresFS embed.FS

func Postgres() fs.FS {
	sub, err := fs.Sub(postgresFS, "postgres")
	if err != nil {
		panic(err)
	}
	return sub
}
