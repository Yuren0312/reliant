// Copyright (c) 2025 Reliant Labs
package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/reliant-labs/reliant/internal/models/message"
)

// summarizeOverflowAttempts bounds the summarization requests one compaction
// makes when the provider refuses the history as too long. Each refusal is a
// fast, unbilled 400, and each retry shrinks the history by the ratio the
// refusal reported (or by half when it reported none), so two shrinks reach
// any realistic window.
const summarizeOverflowAttempts = 3

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

// summarizationHeadShare is the most of the shrunk budget the conversation's
// opening turn may take. That turn is usually the task itself, the one thing a
// handoff cannot do without, so it is kept (cut to this share) while the turns
// after it are dropped oldest first.
const summarizationHeadShare = 0.2

// shrinkSummarizationHistory cuts a flattened, text-only summarization history
// (see prepareHistoryForLLM) to about keep of its characters. It keeps the
// opening turn, the newest turns that fit, and the final summarization
// instruction, and puts a note where the omitted turns were so the summary
// says what it could not see. ok is false when nothing could be cut.
//
// Dropping whole messages is safe here and nowhere else: the history carries
// no tool_use/tool_result blocks left to orphan.
func shrinkSummarizationHistory(history []message.Message, keep float64) ([]message.Message, bool) {
	if len(history) < 2 || keep <= 0 || keep >= 1 {
		return nil, false
	}
	total := 0
	for _, m := range history {
		total += len(m.Content().Text)
	}
	budget := int(float64(total) * keep)

	instruction := history[len(history)-1]
	head := history[0]
	body := history[1 : len(history)-1]

	headText := head.Content().Text
	if maxHead := int(float64(budget) * summarizationHeadShare); len(headText) > maxHead {
		headText = truncateMiddle(headText, maxHead)
	}
	remaining := budget - len(headText) - len(instruction.Content().Text)

	// The newest turns that fit, contiguous from the end. When not even the
	// newest one fits whole it is cut to the room left, so a single enormous
	// turn still shrinks.
	var tail []message.Message
	start := len(body)
	for i := len(body) - 1; i >= 0 && remaining > 0; i-- {
		text := body[i].Content().Text
		if len(text) > remaining {
			if len(tail) == 0 {
				tail = append(tail, textMessage(body[i], truncateMiddle(text, remaining)))
				start = i
			}
			break
		}
		tail = append(tail, body[i])
		remaining -= len(text)
		start = i
	}

	out := make([]message.Message, 0, len(tail)+3)
	out = append(out, textMessage(head, headText))
	if start > 0 {
		out = append(out, message.Message{
			Role: message.User,
			Parts: []message.ContentPart{message.TextContent{Text: fmt.Sprintf(
				"[%d earlier messages omitted: the conversation is larger than the summarizer's context window. "+
					"Summarize from what remains, and say in the handoff that earlier detail was not visible.]", start)}},
		})
	}
	for i := len(tail) - 1; i >= 0; i-- {
		out = append(out, tail[i])
	}
	out = append(out, instruction)

	kept := 0
	for _, m := range out {
		kept += len(m.Content().Text)
	}
	if kept >= total {
		return nil, false
	}
	return out, true
}

// textMessage is m with its content replaced by text.
func textMessage(m message.Message, text string) message.Message {
	m.Parts = []message.ContentPart{message.TextContent{Text: text}}
	return m
}

// truncateMiddle cuts s to at most maxBytes bytes, keeping its head and tail
// around a marker, on rune boundaries.
func truncateMiddle(s string, maxBytes int) string {
	const marker = "\n\n... [omitted to fit the summarizer's context window] ...\n\n"
	if len(s) <= maxBytes {
		return s
	}
	if maxBytes <= len(marker) {
		return marker
	}
	half := (maxBytes - len(marker)) / 2
	headEnd := half
	for headEnd > 0 && !utf8.RuneStart(s[headEnd]) {
		headEnd--
	}
	tailStart := len(s) - half
	for tailStart < len(s) && !utf8.RuneStart(s[tailStart]) {
		tailStart++
	}
	return s[:headEnd] + marker + s[tailStart:]
}
