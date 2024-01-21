package archiver

import "errors"

var (
	ErrParsingArchiverConfig = errors.New("error pasing archiver config")
	ErrNoopArchiver          = errors.New("cannot operate noop archiver")
)
