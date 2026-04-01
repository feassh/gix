package config

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

func ParseTOML(r io.Reader) (Values, error) {
	values := make(Values)
	scanner := bufio.NewScanner(r)
	section := ""
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid toml line %d", lineNo)
		}
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)
		fullKey := key
		if section != "" {
			fullKey = section + "." + key
		}
		spec, known := Schema[normalizeKey(fullKey)]
		if !known {
			continue
		}
		value, err := parseScalar(rawValue, spec.Kind)
		if err != nil {
			return nil, fmt.Errorf("invalid %s at line %d: %w", fullKey, lineNo, err)
		}
		values[normalizeKey(fullKey)] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func EncodeTOML(values Values) string {
	grouped := map[string]map[string]string{}
	for _, key := range values.SortedKeys() {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 {
			continue
		}
		section := parts[0]
		name := parts[1]
		if _, ok := grouped[section]; !ok {
			grouped[section] = map[string]string{}
		}
		grouped[section][name] = values[key]
	}

	sections := make([]string, 0, len(grouped))
	for section := range grouped {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	var b strings.Builder
	for idx, section := range sections {
		b.WriteString("[" + section + "]\n")
		keys := make([]string, 0, len(grouped[section]))
		for key := range grouped[section] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fullKey := section + "." + key
			spec := Schema[fullKey]
			b.WriteString(key)
			b.WriteString(" = ")
			b.WriteString(formatScalar(grouped[section][key], spec.Kind))
			b.WriteString("\n")
		}
		if idx < len(sections)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func parseScalar(raw string, kind ValueKind) (string, error) {
	switch kind {
	case StringKind:
		if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
			value, err := strconv.Unquote(raw)
			if err != nil {
				return "", err
			}
			return value, nil
		}
		return raw, nil
	case BoolKind:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return "", err
		}
		return strconv.FormatBool(value), nil
	case IntKind:
		value, err := strconv.Atoi(raw)
		if err != nil {
			return "", err
		}
		return strconv.Itoa(value), nil
	default:
		return "", fmt.Errorf("unsupported scalar type")
	}
}

func formatScalar(raw string, kind ValueKind) string {
	switch kind {
	case StringKind:
		return strconv.Quote(raw)
	default:
		return raw
	}
}
