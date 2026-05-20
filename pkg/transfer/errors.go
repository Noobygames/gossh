package transfer

import "fmt"

// ErrAborted is returned when the user explicitly aborts an operation at a conflict prompt.
var ErrAborted = fmt.Errorf("aborted by user")

// PathTraversalError is returned when a tar entry would write outside the destination directory.
type PathTraversalError struct {
	Entry string
}

func (e *PathTraversalError) Error() string {
	return fmt.Sprintf("tar entry %q escapes destination directory", e.Entry)
}
