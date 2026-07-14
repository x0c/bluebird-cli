package output

import (
	"encoding/json"
	"strings"
)

// ProjectFields 按点路径（如 "watch.id,watch.name"）裁剪 data，只减不增。
// data 是数组时对每个元素分别投影。paths 为空时原样返回 data。
func ProjectFields(data any, paths []string) (any, error) {
	if len(paths) == 0 {
		return data, nil
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}

	split := make([][]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		split = append(split, strings.Split(p, "."))
	}

	return projectNode(generic, split), nil
}

func projectNode(node any, paths [][]string) any {
	switch v := node.(type) {
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = projectNode(item, paths)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for _, path := range paths {
			if val, ok := getPath(v, path); ok {
				setPath(out, path, val)
			}
		}
		return out
	default:
		return node
	}
}

func getPath(node any, path []string) (any, bool) {
	if len(path) == 0 {
		return node, true
	}
	m, ok := node.(map[string]any)
	if !ok {
		return nil, false
	}
	child, ok := m[path[0]]
	if !ok {
		return nil, false
	}
	return getPath(child, path[1:])
}

func setPath(dst map[string]any, path []string, value any) {
	if len(path) == 1 {
		dst[path[0]] = value
		return
	}
	next, ok := dst[path[0]].(map[string]any)
	if !ok {
		next = map[string]any{}
		dst[path[0]] = next
	}
	setPath(next, path[1:], value)
}
