package completeness

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const (
	sidecarSandboxConfigName protoreflect.FullName = "exa.cortex_pb.SidecarSandboxConfig"
	anyMessageName           protoreflect.FullName = "google.protobuf.Any"
	maxJSONSkipDepth                               = 10000
)

var sandboxAliasReachable map[protoreflect.FullName]bool

// buildSandboxAliasReachable records every non-special message type that can
// transitively reach SidecarSandboxConfig or google.protobuf.Any so RPC JSON
// normalization skips unrelated subtrees (such as google.protobuf.Struct).
func buildSandboxAliasReachable(files *protoregistry.Files) map[protoreflect.FullName]bool {
	var all []protoreflect.MessageDescriptor
	var collect func(protoreflect.MessageDescriptors)
	collect = func(list protoreflect.MessageDescriptors) {
		for i := 0; i < list.Len(); i++ {
			m := list.Get(i)
			if !m.IsMapEntry() {
				all = append(all, m)
			}
			collect(m.Messages())
		}
	}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		collect(fd.Messages())
		return true
	})

	parents := make(map[protoreflect.FullName][]protoreflect.FullName)
	for _, m := range all {
		if specialJSON(m.FullName()) {
			continue
		}
		fields := m.Fields()
		for i := 0; i < fields.Len(); i++ {
			f := fields.Get(i)
			var target protoreflect.MessageDescriptor
			if f.IsMap() {
				if mv := f.MapValue(); mv.Kind() == protoreflect.MessageKind || mv.Kind() == protoreflect.GroupKind {
					target = mv.Message()
				}
			} else if f.Kind() == protoreflect.MessageKind || f.Kind() == protoreflect.GroupKind {
				target = f.Message()
			}
			if target != nil && !specialJSON(target.FullName()) {
				parents[target.FullName()] = append(parents[target.FullName()], m.FullName())
			}
		}
	}

	reachable := map[protoreflect.FullName]bool{
		sidecarSandboxConfigName: true,
		anyMessageName:           true,
	}
	queue := []protoreflect.FullName{sidecarSandboxConfigName, anyMessageName}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, p := range parents[cur] {
			if !reachable[p] {
				reachable[p] = true
				queue = append(queue, p)
			}
		}
	}
	return reachable
}

type jsonSpan struct {
	start int
	end   int
}

type jsonObjectMember struct {
	keySpan jsonSpan
	key     string
	valSpan jsonSpan
}

// normalizeSandboxAllowlist159 rewrites agy 1.5.9's SidecarSandboxConfig field 5
// JSON key ("allowedHosts" / "allowed_hosts") to "allowedHttpHosts" in place
// within the raw JSON token spans, leaving every other byte untouched.
func normalizeSandboxAllowlist159(data []byte, descriptor protoreflect.MessageDescriptor) ([]byte, error) {
	var spans []jsonSpan
	if err := collectSandboxHostAliases(data, jsonSpan{0, len(data)}, descriptor, 0, &spans); err != nil {
		return nil, err
	}
	if len(spans) == 0 {
		return data, nil
	}
	slices.SortFunc(spans, func(a, b jsonSpan) int {
		return cmp.Compare(a.start, b.start)
	})
	const canonicalKey = `"allowedHttpHosts"`
	var out bytes.Buffer
	out.Grow(len(data) + len(spans)*4)
	pos := 0
	for _, s := range spans {
		if s.start < pos || s.end > len(data) || s.start >= s.end {
			return nil, errors.New("invalid trajectory JSON")
		}
		out.Write(data[pos:s.start])
		out.WriteString(canonicalKey)
		pos = s.end
	}
	out.Write(data[pos:])
	return out.Bytes(), nil
}

func collectSandboxHostAliases(data []byte, span jsonSpan, md protoreflect.MessageDescriptor, depth int, spans *[]jsonSpan) error {
	if depth > maxDepth {
		return errors.New("RPC nesting limit")
	}
	name := md.FullName()
	if !sandboxAliasReachable[name] {
		return nil
	}
	if isJSONNull(data[span.start:span.end]) {
		return nil
	}
	if name == anyMessageName {
		members, err := scanJSONObjectWithErr(data, span, "invalid Any JSON")
		if err != nil {
			return err
		}
		if len(members) == 0 {
			return nil
		}
		var typeSpan, valueSpan jsonSpan
		var typeCount, valueCount int
		for _, m := range members {
			switch m.key {
			case "@type":
				typeCount++
				typeSpan = m.valSpan
			case "value":
				valueCount++
				valueSpan = m.valSpan
			}
		}
		if typeCount > 1 {
			return errors.New("invalid Any JSON")
		}
		var typ string
		if typeCount == 1 {
			rawType := bytes.Trim(data[typeSpan.start:typeSpan.end], " \t\n\r")
			if len(rawType) > 0 && rawType[0] == '"' {
				if json.Unmarshal(rawType, &typ) != nil {
					typ = ""
				}
			}
		}
		if typ == "" {
			return errors.New("unsupported Any JSON type")
		}
		mt, err := schemaTypes.FindMessageByURL(typ)
		if err != nil {
			return errors.New("unsupported Any JSON type")
		}
		nested := mt.Descriptor()
		if !sandboxAliasReachable[nested.FullName()] {
			return nil
		}
		if nested.FullName() == anyMessageName {
			if valueCount > 1 {
				return errors.New("invalid Any JSON")
			}
			if valueCount == 0 {
				return nil
			}
			return collectSandboxHostAliases(data, valueSpan, nested, depth+1, spans)
		}
		return collectSandboxHostAliases(data, span, nested, depth+1, spans)
	}

	members, err := scanJSONObjectWithErr(data, span, "invalid RPC message JSON")
	if err != nil {
		return err
	}
	if name == sidecarSandboxConfigName {
		var field5Count, legacyCount int
		var legacySpan jsonSpan
		var hasAllowedRpcs bool
		for _, m := range members {
			switch m.key {
			case "allowedHttpHosts", "allowed_http_hosts":
				field5Count++
			case "allowedHosts", "allowed_hosts":
				field5Count++
				legacyCount++
				legacySpan = m.keySpan
			case "allowedRpcs", "allowed_rpcs":
				hasAllowedRpcs = true
			}
		}
		if legacyCount == 0 {
			return nil
		}
		if field5Count > 1 || hasAllowedRpcs {
			return errors.New("unsupported RPC protobuf JSON")
		}
		*spans = append(*spans, legacySpan)
		return nil
	}

	fields := md.Fields()
	for _, m := range members {
		fd := fields.ByJSONName(m.key)
		if fd == nil {
			fd = fields.ByName(protoreflect.Name(m.key))
		}
		if fd == nil || isJSONNull(data[m.valSpan.start:m.valSpan.end]) {
			continue
		}
		switch {
		case fd.IsMap():
			mv := fd.MapValue()
			if (mv.Kind() != protoreflect.MessageKind && mv.Kind() != protoreflect.GroupKind) || !sandboxAliasReachable[mv.Message().FullName()] {
				continue
			}
			entries, err := scanJSONObjectWithErr(data, m.valSpan, "invalid RPC map")
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err := collectSandboxHostAliases(data, entry.valSpan, mv.Message(), depth+1, spans); err != nil {
					return err
				}
			}
		case fd.IsList():
			if (fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind) || !sandboxAliasReachable[fd.Message().FullName()] {
				continue
			}
			elems, err := scanJSONArray(data, m.valSpan)
			if err != nil {
				return err
			}
			for _, elemSpan := range elems {
				if err := collectSandboxHostAliases(data, elemSpan, fd.Message(), depth+1, spans); err != nil {
					return err
				}
			}
		default:
			if (fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind) || !sandboxAliasReachable[fd.Message().FullName()] {
				continue
			}
			if err := collectSandboxHostAliases(data, m.valSpan, fd.Message(), depth+1, spans); err != nil {
				return err
			}
		}
	}
	return nil
}

func skipJSONSpace(data []byte, pos, end int) int {
	for pos < end {
		switch data[pos] {
		case ' ', '\t', '\n', '\r':
			pos++
		default:
			return pos
		}
	}
	return pos
}

func isJSONNull(raw []byte) bool {
	return bytes.Equal(bytes.Trim(raw, " \t\n\r"), []byte("null"))
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func scanJSONString(data []byte, pos, end int) (int, error) {
	if pos >= end || data[pos] != '"' {
		return 0, errors.New("invalid trajectory JSON")
	}
	i := pos + 1
	for i < end {
		c := data[i]
		if c < 0x20 {
			return 0, errors.New("invalid trajectory JSON")
		}
		if c == '"' {
			return i + 1, nil
		}
		if c == '\\' {
			i++
			if i >= end {
				return 0, errors.New("invalid trajectory JSON")
			}
			switch data[i] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				i++
			case 'u':
				if i+4 >= end || !isHexDigit(data[i+1]) || !isHexDigit(data[i+2]) || !isHexDigit(data[i+3]) || !isHexDigit(data[i+4]) {
					return 0, errors.New("invalid trajectory JSON")
				}
				i += 5
			default:
				return 0, errors.New("invalid trajectory JSON")
			}
			continue
		}
		i++
	}
	return 0, errors.New("invalid trajectory JSON")
}

func scanJSONNumber(data []byte, pos, end int) (int, error) {
	if pos < end && data[pos] == '-' {
		pos++
	}
	if pos >= end {
		return 0, errors.New("invalid trajectory JSON")
	}
	if data[pos] == '0' {
		pos++
		if pos < end && data[pos] >= '0' && data[pos] <= '9' {
			return 0, errors.New("invalid trajectory JSON")
		}
	} else if data[pos] >= '1' && data[pos] <= '9' {
		for pos < end && data[pos] >= '0' && data[pos] <= '9' {
			pos++
		}
	} else {
		return 0, errors.New("invalid trajectory JSON")
	}
	if pos < end && data[pos] == '.' {
		pos++
		if pos >= end || data[pos] < '0' || data[pos] > '9' {
			return 0, errors.New("invalid trajectory JSON")
		}
		for pos < end && data[pos] >= '0' && data[pos] <= '9' {
			pos++
		}
	}
	if pos < end && (data[pos] == 'e' || data[pos] == 'E') {
		pos++
		if pos < end && (data[pos] == '+' || data[pos] == '-') {
			pos++
		}
		if pos >= end || data[pos] < '0' || data[pos] > '9' {
			return 0, errors.New("invalid trajectory JSON")
		}
		for pos < end && data[pos] >= '0' && data[pos] <= '9' {
			pos++
		}
	}
	return pos, nil
}

func scanJSONScalar(data []byte, pos, end int) (int, error) {
	if pos >= end {
		return 0, errors.New("invalid trajectory JSON")
	}
	switch data[pos] {
	case '"':
		return scanJSONString(data, pos, end)
	case 't':
		if pos+4 <= end && string(data[pos:pos+4]) == "true" {
			return pos + 4, nil
		}
		return 0, errors.New("invalid trajectory JSON")
	case 'f':
		if pos+5 <= end && string(data[pos:pos+5]) == "false" {
			return pos + 5, nil
		}
		return 0, errors.New("invalid trajectory JSON")
	case 'n':
		if pos+4 <= end && string(data[pos:pos+4]) == "null" {
			return pos + 4, nil
		}
		return 0, errors.New("invalid trajectory JSON")
	default:
		return scanJSONNumber(data, pos, end)
	}
}

const (
	jsonStateObjKeyOrEnd byte = iota
	jsonStateObjKey
	jsonStateObjColon
	jsonStateObjVal
	jsonStateObjCommaOrEnd
	jsonStateArrValOrEnd
	jsonStateArrVal
	jsonStateArrCommaOrEnd
)

func scanJSONValue(data []byte, pos, end int) (int, int, error) {
	pos = skipJSONSpace(data, pos, end)
	if pos >= end {
		return 0, 0, errors.New("invalid trajectory JSON")
	}
	start := pos
	if data[pos] != '{' && data[pos] != '[' {
		next, err := scanJSONScalar(data, pos, end)
		return start, next, err
	}
	stack := make([]byte, 1, 16)
	if data[pos] == '{' {
		stack[0] = jsonStateObjKeyOrEnd
	} else {
		stack[0] = jsonStateArrValOrEnd
	}
	pos++
	for len(stack) > 0 {
		pos = skipJSONSpace(data, pos, end)
		if pos >= end {
			return 0, 0, errors.New("invalid trajectory JSON")
		}
		topIdx := len(stack) - 1
		switch stack[topIdx] {
		case jsonStateObjKeyOrEnd:
			if data[pos] == '}' {
				stack = stack[:topIdx]
				pos++
				continue
			}
			next, err := scanJSONString(data, pos, end)
			if err != nil {
				return 0, 0, err
			}
			pos = next
			stack[topIdx] = jsonStateObjColon
		case jsonStateObjKey:
			next, err := scanJSONString(data, pos, end)
			if err != nil {
				return 0, 0, err
			}
			pos = next
			stack[topIdx] = jsonStateObjColon
		case jsonStateObjColon:
			if data[pos] != ':' {
				return 0, 0, errors.New("invalid trajectory JSON")
			}
			pos++
			stack[topIdx] = jsonStateObjVal
		case jsonStateObjVal:
			switch data[pos] {
			case '{':
				if len(stack) >= maxJSONSkipDepth {
					return 0, 0, errors.New("RPC nesting limit")
				}
				stack[topIdx] = jsonStateObjCommaOrEnd
				stack = append(stack, jsonStateObjKeyOrEnd)
				pos++
			case '[':
				if len(stack) >= maxJSONSkipDepth {
					return 0, 0, errors.New("RPC nesting limit")
				}
				stack[topIdx] = jsonStateObjCommaOrEnd
				stack = append(stack, jsonStateArrValOrEnd)
				pos++
			default:
				next, err := scanJSONScalar(data, pos, end)
				if err != nil {
					return 0, 0, err
				}
				pos = next
				stack[topIdx] = jsonStateObjCommaOrEnd
			}
		case jsonStateObjCommaOrEnd:
			switch data[pos] {
			case '}':
				stack = stack[:topIdx]
				pos++
			case ',':
				pos++
				stack[topIdx] = jsonStateObjKey
			default:
				return 0, 0, errors.New("invalid trajectory JSON")
			}
		case jsonStateArrValOrEnd:
			if data[pos] == ']' {
				stack = stack[:topIdx]
				pos++
				continue
			}
			fallthrough
		case jsonStateArrVal:
			switch data[pos] {
			case '{':
				if len(stack) >= maxJSONSkipDepth {
					return 0, 0, errors.New("RPC nesting limit")
				}
				stack[topIdx] = jsonStateArrCommaOrEnd
				stack = append(stack, jsonStateObjKeyOrEnd)
				pos++
			case '[':
				if len(stack) >= maxJSONSkipDepth {
					return 0, 0, errors.New("RPC nesting limit")
				}
				stack[topIdx] = jsonStateArrCommaOrEnd
				stack = append(stack, jsonStateArrValOrEnd)
				pos++
			default:
				next, err := scanJSONScalar(data, pos, end)
				if err != nil {
					return 0, 0, err
				}
				pos = next
				stack[topIdx] = jsonStateArrCommaOrEnd
			}
		case jsonStateArrCommaOrEnd:
			switch data[pos] {
			case ']':
				stack = stack[:topIdx]
				pos++
			case ',':
				pos++
				stack[topIdx] = jsonStateArrVal
			default:
				return 0, 0, errors.New("invalid trajectory JSON")
			}
		}
	}
	return start, pos, nil
}

func scanJSONObjectWithErr(data []byte, span jsonSpan, nonObjectErr string) ([]jsonObjectMember, error) {
	pos := skipJSONSpace(data, span.start, span.end)
	if pos >= span.end {
		return nil, errors.New("invalid trajectory JSON")
	}
	if data[pos] != '{' {
		return nil, errors.New(nonObjectErr)
	}
	pos = skipJSONSpace(data, pos+1, span.end)
	if pos >= span.end {
		return nil, errors.New("invalid trajectory JSON")
	}
	if data[pos] == '}' {
		if skipJSONSpace(data, pos+1, span.end) != span.end {
			return nil, errors.New("invalid trajectory JSON")
		}
		return nil, nil
	}
	var members []jsonObjectMember
	for {
		keyEnd, err := scanJSONString(data, pos, span.end)
		if err != nil {
			return nil, err
		}
		var key string
		if json.Unmarshal(data[pos:keyEnd], &key) != nil {
			return nil, errors.New("invalid trajectory JSON")
		}
		colon := skipJSONSpace(data, keyEnd, span.end)
		if colon >= span.end || data[colon] != ':' {
			return nil, errors.New("invalid trajectory JSON")
		}
		valStart, valEnd, err := scanJSONValue(data, colon+1, span.end)
		if err != nil {
			return nil, err
		}
		members = append(members, jsonObjectMember{
			keySpan: jsonSpan{pos, keyEnd},
			key:     key,
			valSpan: jsonSpan{valStart, valEnd},
		})
		pos = skipJSONSpace(data, valEnd, span.end)
		if pos >= span.end {
			return nil, errors.New("invalid trajectory JSON")
		}
		if data[pos] == '}' {
			if skipJSONSpace(data, pos+1, span.end) != span.end {
				return nil, errors.New("invalid trajectory JSON")
			}
			return members, nil
		}
		if data[pos] != ',' {
			return nil, errors.New("invalid trajectory JSON")
		}
		pos = skipJSONSpace(data, pos+1, span.end)
	}
}

func scanJSONArray(data []byte, span jsonSpan) ([]jsonSpan, error) {
	pos := skipJSONSpace(data, span.start, span.end)
	if pos >= span.end {
		return nil, errors.New("invalid trajectory JSON")
	}
	if data[pos] != '[' {
		return nil, errors.New("invalid RPC list")
	}
	pos = skipJSONSpace(data, pos+1, span.end)
	if pos >= span.end {
		return nil, errors.New("invalid trajectory JSON")
	}
	if data[pos] == ']' {
		if skipJSONSpace(data, pos+1, span.end) != span.end {
			return nil, errors.New("invalid trajectory JSON")
		}
		return nil, nil
	}
	var elems []jsonSpan
	for {
		valStart, valEnd, err := scanJSONValue(data, pos, span.end)
		if err != nil {
			return nil, err
		}
		elems = append(elems, jsonSpan{valStart, valEnd})
		pos = skipJSONSpace(data, valEnd, span.end)
		if pos >= span.end {
			return nil, errors.New("invalid trajectory JSON")
		}
		if data[pos] == ']' {
			if skipJSONSpace(data, pos+1, span.end) != span.end {
				return nil, errors.New("invalid trajectory JSON")
			}
			return elems, nil
		}
		if data[pos] != ',' {
			return nil, errors.New("invalid trajectory JSON")
		}
		pos = skipJSONSpace(data, pos+1, span.end)
	}
}
