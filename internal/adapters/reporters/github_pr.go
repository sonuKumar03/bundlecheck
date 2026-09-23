package reporters

import (
	"bytes"
	"context"
	"io"
)

type GitHubPRReporter struct {
	md MarkdownReporter
}

func (r *GitHubPRReporter) Format() string {
	return "github-pr"
}

func (r *GitHubPRReporter) Render(ctx context.Context, w io.Writer, data any) error {
	var buf bytes.Buffer
	buf.WriteString("<!-- bundleradar-report -->\n")
	if err := r.md.Render(ctx, &buf, data); err != nil {
		return err
	}
	_, err := io.Copy(w, &buf)
	return err
}
