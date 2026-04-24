package schwab

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Authorizer drives the interactive OAuth2 code-exchange step. Given the
// authorize URL, an Authorizer must return either the full redirect URL
// (with ?code=...) or the raw authorization code.
type Authorizer interface {
	// Authorize is called once per authorization attempt.
	Authorize(ctx context.Context, authorizeURL string) (redirectOrCode string, err error)
}

// BrowserAuthorizer opens the default browser to the authorize URL and reads
// the pasted redirect URL from stdin. This is the default Authorizer when no
// override is supplied — direct parity with Schwabdev's tokens.call_for_auth.
type BrowserAuthorizer struct {
	// Prompt is where status messages are printed. Defaults to os.Stderr.
	Prompt io.Writer
	// Input is where the pasted URL is read from. Defaults to os.Stdin.
	Input io.Reader
}

// Authorize implements Authorizer.
func (b *BrowserAuthorizer) Authorize(ctx context.Context, authorizeURL string) (string, error) {
	prompt := b.Prompt
	if prompt == nil {
		prompt = os.Stderr
	}
	input := b.Input
	if input == nil {
		input = os.Stdin
	}
	// Best-effort browser launch — failure is not fatal.
	if err := openBrowser(authorizeURL); err != nil {
		fmt.Fprintf(prompt, "Could not open browser automatically (%v).\n", err)
	}
	fmt.Fprintf(prompt, "Open this URL in your browser to authorize:\n  %s\n", authorizeURL)
	fmt.Fprint(prompt, "After authorizing, paste the full redirect URL here and press Enter:\n> ")

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		reader := bufio.NewReader(input)
		line, err := reader.ReadString('\n')
		ch <- result{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		if r.err != nil && r.err != io.EOF {
			return "", fmt.Errorf("schwab: read redirect URL: %w", r.err)
		}
		out := strings.TrimSpace(r.line)
		if out == "" {
			return "", fmt.Errorf("schwab: empty redirect URL")
		}
		return out, nil
	}
}

// FuncAuthorizer adapts a plain function into the Authorizer interface.
type FuncAuthorizer func(ctx context.Context, authorizeURL string) (string, error)

// Authorize implements Authorizer by delegating to the underlying function.
func (f FuncAuthorizer) Authorize(ctx context.Context, authorizeURL string) (string, error) {
	return f(ctx, authorizeURL)
}

// openBrowser tries to open url in the user's default browser. It uses the
// platform-native helper (open/xdg-open/rundll32) without third-party deps.
func openBrowser(url string) error {
	var (
		cmd  string
		args []string
	)
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default: // linux, freebsd, etc.
		cmd = "xdg-open"
		args = []string{url}
	}
	return exec.Command(cmd, args...).Start()
}
