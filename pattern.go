package websocket

import (
	"strings"

	"github.com/grafana/regexp"
)

type Pattern struct {
	str    string
	chunks []chunk
	regExp *regexp.Regexp
}

func NewPattern(patternStr string) (*Pattern, error) {
	chunks, err := parsePatternChunks(patternStr)
	if err != nil {
		return nil, err
	}

	patternRegExp, err := regExpFromChunks(chunks)
	if err != nil {
		return nil, err
	}

	return &Pattern{
		str:    patternStr,
		chunks: chunks,
		regExp: patternRegExp,
	}, nil
}

func (p *Pattern) Match(event string) bool {
	return p.regExp.MatchString(event)
}

func (p *Pattern) String() string {
	return p.str
}

type chunkKind int

const (
	unknown chunkKind = iota
	static
	dynamic
	wildcard
)

type chunkModifier int

const (
	single chunkModifier = iota
	optional
	oneOrMore
	zeroOrMore
)

type chunk struct {
	kind     chunkKind
	modifier chunkModifier
	key      string
	pattern  string
}

func parsePatternChunks(patternStr string) ([]chunk, error) {
	parts := strings.Split(patternStr, ".")
	chunks := make([]chunk, 0, len(parts))

	for _, part := range parts {
		if part == "" {
			continue
		}
		ch := chunk{kind: static, modifier: single}
		switch part {
		case "*":
			ch.kind = wildcard
			ch.modifier = single
			ch.pattern = "[^.]+"
		case "**":
			ch.kind = wildcard
			ch.modifier = zeroOrMore
			ch.pattern = ".*"
		default:
			ch.pattern = regexp.QuoteMeta(part)
		}

		chunks = append(chunks, ch)
	}

	return chunks, nil
}

func regExpFromChunks(chunks []chunk) (*regexp.Regexp, error) {
	if len(chunks) == 0 {
		return regexp.Compile("^$")
	}

	var regExpStr strings.Builder
	regExpStr.WriteString("^")
	for i, currentChunk := range chunks {
		if i > 0 {
			regExpStr.WriteString("\\.")
		}
		switch currentChunk.modifier {
		case single:
			regExpStr.WriteString(currentChunk.pattern)
		case zeroOrMore:
			regExpStr.WriteString(currentChunk.pattern)
		}
	}

	regExpStr.WriteString("$")
	return regexp.Compile(regExpStr.String())
}
