package warehouse

import "errors"

// Sentinel errors for the warehouse package.
var (
	ErrNoRowsToWrite = errors.New("no rows to write")
)
