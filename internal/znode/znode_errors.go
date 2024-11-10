package znode

// This file contains custom errors for the znode package
// Meant for use with errors.As() to check for specific errors (or just type check)

type InvalidRequestError struct {
	Msg string
}

func (e *InvalidRequestError) Error() string {
	return e.Msg
}

type ExistsError struct {
	Msg string
}

func (e *ExistsError) Error() string {
	return e.Msg
}

type VersionError struct {
	Msg string
}

func (e *VersionError) Error() string {
	return e.Msg
}

type CriticalError struct {
	Msg string
}

func (e *CriticalError) Error() string {
	return e.Msg
}
