package mcpserver

import "io"

// Test exports — visible only in _test packages.

// StdioReaderFunc is the test-visible alias for stdioReaderFunc so external
// test packages can swap the reader (e.g., to inject a pipe that returns EOF).
var StdioReaderFunc = &stdioReaderFunc

// StdioWriterFunc is the test-visible alias for stdioWriterFunc so external
// test packages can swap the writer (e.g., to discard output during tests).
var StdioWriterFunc = &stdioWriterFunc

// SetStdioIO replaces both reader and writer for a test and returns a cleanup func.
func SetStdioIO(r func() io.Reader, w func() io.Writer) (restore func()) {
	oldR := stdioReaderFunc
	oldW := stdioWriterFunc
	stdioReaderFunc = r
	stdioWriterFunc = w
	return func() {
		stdioReaderFunc = oldR
		stdioWriterFunc = oldW
	}
}
