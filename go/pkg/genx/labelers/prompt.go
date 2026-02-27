package labelers

import (
	"fmt"
	"strings"
)

func buildPrompt(input Input) string {
	var sb strings.Builder
	sb.WriteString(promptBase)
	sb.WriteString("\n\n## Query\n")
	sb.WriteString(input.Text)
	sb.WriteString("\n\n## Candidates\n")
	for _, c := range input.Candidates {
		sb.WriteString("- ")
		sb.WriteString(c)
		if aliases := input.Aliases[c]; len(aliases) > 0 {
			sb.WriteString(" (aliases: ")
			sb.WriteString(strings.Join(aliases, ", "))
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}

	topK := input.TopK
	if topK <= 0 || topK > len(input.Candidates) {
		topK = len(input.Candidates)
	}
	if topK < 0 {
		topK = 0
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(promptOutputFormat, topK))
	return sb.String()
}

const promptBase = `You are a query label selector for memory recall.

Select labels from the provided candidates that are explicitly relevant to the query.

Rules:
- You MUST choose labels only from candidates.
- Prefer precision over recall; do not guess unsupported labels.
- If nothing is relevant, return an empty matches list.
- score must be in [0, 1].`

const promptOutputFormat = `## Output

Call the provided function with JSON arguments:

{
  "matches": [
    {"label": "person:小明", "score": 0.95}
  ]
}

Limit the number of matches to at most %d.`
