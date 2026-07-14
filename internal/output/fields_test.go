package output

import (
	"reflect"
	"testing"
)

func TestProjectFields_NoPaths_ReturnsOriginal(t *testing.T) {
	data := map[string]any{"a": 1}
	got, err := ProjectFields(data, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, data) {
		t.Fatalf("got %v, want unchanged %v", got, data)
	}
}

func TestProjectFields_ObjectProjection(t *testing.T) {
	type inner struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type outer struct {
		Watch inner `json:"watch"`
		Total int   `json:"total"`
	}
	data := outer{Watch: inner{ID: "w1", Name: "杭州西湖"}, Total: 1}

	got, err := ProjectFields(data, []string{"watch.id", "total"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]any{
		"watch": map[string]any{"id": "w1"},
		"total": float64(1),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestProjectFields_ArrayAppliesPerElement(t *testing.T) {
	data := []map[string]any{
		{"id": "1", "name": "a", "score": 0.9},
		{"id": "2", "name": "b", "score": 0.5},
	}
	got, err := ProjectFields(data, []string{"id", "name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []any{
		map[string]any{"id": "1", "name": "a"},
		map[string]any{"id": "2", "name": "b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestProjectFields_MissingPathIsSkipped(t *testing.T) {
	data := map[string]any{"a": 1}
	got, err := ProjectFields(data, []string{"a", "b.c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{"a": float64(1)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
