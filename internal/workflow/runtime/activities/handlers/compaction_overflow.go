// Copyright (c) 2025 Reliant Labs
package handlers

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/reliant-labs/reliant/internal/models/message"
)

// A prefix below this size is not a useful compaction. Refusals at this
// minimum skip compaction, leaving the existing window available for the next
// turn. Every retry at least halves the variable input, so reaching the minimum
// takes at most logarithmically many requests, regardless of token density.
const minimumSummaryPrefixBytes = 1024

var errCompactionOverflow = errors.New("compaction skipped: summarizer cannot fit a useful prefix; original context retained")

// overflowSafetyMargin scales the shrink below the exact ratio a refusal
// reports: characters are only a proxy for the provider's tokens, the system
// prompt and the omission note ride on top, and the response needs room too.
const overflowSafetyMargin = 0.8

// overflowFallbackKeep is the fraction of the history kept after a refusal
// that names no token counts.
const overflowFallbackKeep = 0.5

// contextOverflow is a provider's refusal of a prompt as larger than the
// model's window. Actual and Limit are 0 when the message names no counts.
type contextOverflow struct {
	Actual int
	Limit  int
}

// keepFraction is the share of the history's characters the retry may keep.
func (o contextOverflow) keepFraction() float64 {
	if o.Actual > 0 && o.Limit > 0 && o.Actual > o.Limit {
		return float64(o.Limit) / float64(o.Actual) * overflowSafetyMargin
	}
	return overflowFallbackKeep
}

// overflowCounts match the refusals that state both counts. Each pattern's
// groups are named so the order a provider prints them in does not matter.
var overflowCounts = []*regexp.Regexp{
	// Anthropic: "prompt is too long: 1114231 tokens > 1000000 maximum"
	regexp.MustCompile(`prompt is too long: (?P<actual>\d+) tokens > (?P<limit>\d+) maximum`),
	// OpenAI chat completions: "This model's maximum context length is 128000
	// tokens. However, your messages resulted in 130000 tokens."
	regexp.MustCompile(`maximum context length is (?P<limit>\d+) tokens.*?resulted in (?P<actual>\d+) tokens`),
	// Gemini: "The input token count (1200000) exceeds the maximum number of
	// tokens allowed (1048576)."
	regexp.MustCompile(`input token count \((?P<actual>\d+)\) exceeds the maximum number of tokens allowed \((?P<limit>\d+)\)`),
}

// overflowMarkers identify a context-window refusal that states no counts
// (OpenAI Responses / codex: "Your input exceeds the context window of this
// model", code context_length_exceeded).
var overflowMarkers = []string{
	"prompt is too long",
	"context_length_exceeded",
	"exceeds the context window",
	"maximum context length",
	"input is too long",
}

// parseContextOverflow reports whether err is a provider refusing the prompt as
// larger than the model's context window, with the token counts it gave.
func parseContextOverflow(err error) (contextOverflow, bool) {
	if err == nil {
		return contextOverflow{}, false
	}
	text := err.Error()
	for _, re := range overflowCounts {
		m := re.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		actual, aerr := strconv.Atoi(m[re.SubexpIndex("actual")])
		limit, lerr := strconv.Atoi(m[re.SubexpIndex("limit")])
		if aerr == nil && lerr == nil {
			return contextOverflow{Actual: actual, Limit: limit}, true
		}
	}
	lower := strings.ToLower(text)
	for _, marker := range overflowMarkers {
		if strings.Contains(lower, marker) {
			return contextOverflow{}, true
		}
	}
	return contextOverflow{}, false
}

// compactionTranscript is lossless for the text/tool content the summarizer
// consumes. Include roles and call IDs so the unprocessed suffix remains a
// useful continuation, including results and still-pending tool calls. Unlike
// prepareHistoryForLLM this read-only path never trims large tool results.
func compactionTranscript(messages []message.Message) string {
	var out strings.Builder
	for _, m := range messages {
		fmt.Fprintf(&out, "\n[%s]\n", m.Role)
		for _, text := range m.TextContents() {
			out.WriteString(text.Text)
			out.WriteByte('\n')
		}
		for _, call := range m.ToolCalls() {
			fmt.Fprintf(&out, "[Tool call %s %s input: %s]\n", call.ID, call.Name, call.Input)
		}
		for _, result := range m.ToolResults() {
			fmt.Fprintf(&out, "[Tool result %s %s error=%t: %s]\n", result.ToolCallID, result.Name, result.IsError, result.Content)
		}
	}
	return out.String()
}

// summaryPrefixEnd splits without losing even a byte of the unprocessed tail.
// Budgets smaller than a rune yield an empty prefix, never an oversized marker.
func summaryPrefixEnd(transcript string, budget int) int {
	end := min(max(budget, 0), len(transcript))
	for end > 0 && end < len(transcript) && !utf8.RuneStart(transcript[end]) {
		end--
	}
	return end
}

// nextSummaryPrefixBudget reserves all fixed text before scaling the variable
// prefix. Counts are only a hint: cap the keep fraction at half to guarantee
// bounded progress even with misleading counts or changing token density.
func nextSummaryPrefixBudget(current, overhead int, overflow contextOverflow) int {
	if current <= minimumSummaryPrefixBytes {
		return 0
	}
	next := int(float64(current+overhead)*min(overflow.keepFraction(), overflowFallbackKeep)) - overhead
	return max(next, minimumSummaryPrefixBytes)
}

const unprocessedContinuation = "\n\nThe summary above covers only the processed prefix. The following is the verbatim unprocessed continuation; preserve its user instructions, completed results and pending tool calls:\n"
