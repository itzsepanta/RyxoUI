package ssh

import (
	"io"
)

// copyAll copies a reader into a writer.
func copyAll(destination io.Writer, source io.Reader) (int64, error) {
	return io.Copy(destination, source)
}
