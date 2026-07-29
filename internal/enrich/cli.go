package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// cliClient generates card content by shelling out to the local `claude`
// command (Claude Code CLI) in non-interactive print mode. This reuses the
// user's existing Claude login, so no ANTHROPIC_API_KEY is needed.
type cliClient struct {
	path  string // executable, e.g. "claude"
	model string // optional model name/alias passed via --model
}

func newCLIClient(path, model string) *cliClient {
	if path == "" {
		path = "claude"
	}
	return &cliClient{path: path, model: model}
}

// cliEnvelope is the JSON object `claude -p --output-format json` prints. The
// model's own answer is carried in the "result" string.
type cliEnvelope struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
}

// generate runs the CLI with the prompt on stdin and parses the JSON the model
// returns. A per-call timeout guards against a hung subprocess.
func (c *cliClient) generate(ctx context.Context, word, contextSentence string) (*llmResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	// Fold the system instructions into the prompt so we don't depend on a
	// specific system-prompt flag; the CLI just needs a prompt that asks for JSON.
	prompt := systemPrompt + "\n\n" + buildUserPrompt(word, contextSentence)

	args := []string{"-p", "--output-format", "json"}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	cmd := exec.CommandContext(ctx, c.path, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("claude cli timed out")
		}
		// On failure the CLI usually exits non-zero with an empty stderr and the
		// real reason inside stdout's JSON envelope (e.g. "Failed to
		// authenticate: OAuth session expired"). Surface that instead of a bare
		// exit status.
		if msg := envelopeError(stdout.Bytes()); msg != "" {
			return nil, fmt.Errorf("claude cli: %s", msg)
		}
		return nil, fmt.Errorf("claude cli: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return parseCLIOutput(stdout.Bytes())
}

// envelopeError extracts the error text from a CLI JSON envelope, or "" if the
// output is not an envelope / carries no message.
func envelopeError(out []byte) string {
	var env cliEnvelope
	if err := json.Unmarshal(bytes.TrimSpace(out), &env); err != nil {
		return ""
	}
	return strings.TrimSpace(env.Result)
}

// parseCLIOutput unwraps the CLI JSON envelope and then extracts the model's
// JSON object from the result text. It tolerates the CLI printing plain text
// (no envelope) by falling back to parsing the raw output.
func parseCLIOutput(out []byte) (*llmResult, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("claude cli: empty output")
	}

	var env cliEnvelope
	if err := json.Unmarshal(trimmed, &env); err == nil && env.Result != "" {
		if env.IsError {
			return nil, fmt.Errorf("claude cli reported error: %s", env.Result)
		}
		return parseLLMJSON(env.Result)
	}

	// Not the expected envelope (e.g. --output-format text): parse directly.
	return parseLLMJSON(string(trimmed))
}
