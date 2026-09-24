package plan

import (
	"bytes"
	"errors"
)

const (
	beginMarker = "<!-- rubric:begin -->"
	endMarker   = "<!-- rubric:end -->"
)

var errMarkers = errors.New("malformed rubric markers; fix AGENTS.md by hand so it has one begin marker before one end marker")

type sections struct {
	before, managed, after []byte
	found                  bool
}

func splitGuidance(data []byte) (sections, error) {
	begins, ends := bytes.Count(data, []byte(beginMarker)), bytes.Count(data, []byte(endMarker))
	if begins == 0 && ends == 0 {
		return sections{before: data}, nil
	}
	start, stop := bytes.Index(data, []byte(beginMarker)), bytes.Index(data, []byte(endMarker))
	if begins != 1 || ends != 1 || stop < start {
		return sections{}, errMarkers
	}
	stop += len(endMarker)
	if stop < len(data) && data[stop] == '\n' {
		stop++
	}
	return sections{before: data[:start], managed: data[start:stop], after: data[stop:], found: true}, nil
}

func (s sections) with(section []byte) []byte {
	var buf bytes.Buffer
	buf.Write(s.before)
	if !s.found && len(s.before) > 0 {
		if !bytes.HasSuffix(s.before, []byte("\n")) {
			buf.WriteByte('\n')
		}
		buf.WriteByte('\n')
	}
	buf.Write(section)
	buf.Write(s.after)
	return buf.Bytes()
}
