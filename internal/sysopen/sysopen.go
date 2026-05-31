// Package sysopen launches the OS default handler for a URL or file path
// (default browser, text editor, etc.) without blocking the caller.
package sysopen

// Open launches the OS default handler for target, which may be a URL
// (http://…) or a local file/folder path. It returns once the handler has been
// launched; it does not wait for the handler to exit.
func Open(target string) error {
	return command(target).Start()
}
