package writ

import (
	"encoding/json"
	"log/slog"
	"strings"
)

// Custom unmarshaller for the ForwardPort type
func (f *ForwardPort) UnmarshalJSON(data []byte) error {
	if len(data) < 1 {
		return nil
	}

	var err error
	if data[0] == '"' {
		var hostPort string
		if err = json.Unmarshal(data, &hostPort); err == nil {
			f.String = &hostPort
		}
	} else {
		var port int64
		if err = json.Unmarshal(data, &port); err == nil {
			f.Integer = &port
		}
	}
	return err
}

// Custom unmarshaller for the MountElement type
//
// Because of this unmarshaller, a MountElement should never have its
// String field be non-nil with a valid decontainer.json file
func (m *MountElement) UnmarshalJSON(data []byte) error {
	m.Mount = &Mount{}
	if len(data) > 0 && data[0] == '{' {
		return json.Unmarshal(data, m.Mount)
	}

	var mountString string
	if err := json.Unmarshal(data, &mountString); err != nil {
		return err
	}

	for segment := range strings.SplitSeq(mountString, ",") {
		splitSegment := strings.SplitN(segment, "=", 2)
		splitSegment[0] = strings.ToLower(strings.TrimSpace(splitSegment[0]))
		splitSegment[1] = strings.TrimSpace(splitSegment[1])

		switch splitSegment[0] {
		case "source":
			m.Mount.Source = splitSegment[1]
		case "target":
			m.Mount.Target = splitSegment[1]
		case "type":
			m.Mount.Type = Type(strings.ToLower(splitSegment[1]))
		default:
			slog.Debug("ignoring unknown mount directive", "key", splitSegment[0], "value", splitSegment[1])
		}
	}
	return nil
}
