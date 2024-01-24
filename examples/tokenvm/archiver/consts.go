package archiver

import "errors"

var (
	ErrParsingArchiverConfig   = errors.New("error pasing archiver config")
	ErrNoopArchiver            = errors.New("cannot operate noop archiver")
	ErrSpecifiedLocationNotDir = errors.New("specified location is not a directory")
	ErrSpecifiedFileIsADir     = errors.New("specified location is a directory")
)
