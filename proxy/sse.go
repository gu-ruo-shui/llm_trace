package proxy

import "strings"

type parsedSSEEvent struct {
	EventType string
	Data      string
}

type sseEventParser struct {
	eventType string
	dataLines []string
}

func (p *sseEventParser) ProcessLine(rawLine string) (parsedSSEEvent, bool) {
	line := strings.TrimRight(rawLine, "\r\n")
	if line == "" {
		return p.dispatch()
	}

	// SSE comments/heartbeats start with ':' and are not dispatched as events.
	if strings.HasPrefix(line, ":") {
		return parsedSSEEvent{}, false
	}

	field, value, hasColon := strings.Cut(line, ":")
	if hasColon && strings.HasPrefix(value, " ") {
		value = value[1:]
	}
	if !hasColon {
		value = ""
	}

	switch field {
	case "event":
		p.eventType = value
	case "data":
		p.dataLines = append(p.dataLines, value)
	}

	return parsedSSEEvent{}, false
}

func (p *sseEventParser) Flush() (parsedSSEEvent, bool) {
	return p.dispatch()
}

func (p *sseEventParser) dispatch() (parsedSSEEvent, bool) {
	if len(p.dataLines) == 0 {
		p.eventType = ""
		return parsedSSEEvent{}, false
	}

	eventType := p.eventType
	if eventType == "" {
		eventType = "message"
	}

	event := parsedSSEEvent{
		EventType: eventType,
		Data:      strings.Join(p.dataLines, "\n"),
	}

	p.eventType = ""
	p.dataLines = nil
	return event, true
}
