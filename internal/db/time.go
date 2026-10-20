package db

import "time"

// nowUnix returns the current time as Unix seconds, the representation used
// for every timestamp column in the schema.
func nowUnix() int64 { return time.Now().UTC().Unix() }

// Now exposes the schema timestamp representation to other packages.
func Now() int64 { return nowUnix() }
