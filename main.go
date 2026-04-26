package main

import (
	"strconv"
	"time"

	"github.com/cryptolink/cryptolink/cmd"
	"github.com/samber/lo"
)

func init() {
	// Force UTC everywhere to prevent timezone mismatch with PostgreSQL.
	// pgx v4 writes timestamp without time zone using time.Format() (uses local TZ)
	// but reads using time.Parse() (defaults to UTC). On a CET server this causes
	// a 1-hour shift between written and read values, breaking expiration checks.
	time.Local = time.UTC
}

// set by LDFLAGS at compile time
var (
	gitCommit     string
	gitVersion    string
	embedFrontend string
)

func main() {
	cmd.Version = gitVersion
	cmd.Commit = gitCommit
	cmd.EmbedFrontend = lo.Must(strconv.ParseBool(embedFrontend))

	cmd.Execute()
}
