package watch

import (
	"testing"

	"github.com/x0c/bluebird-cli/internal/api"
)

func TestSameLocation_RoundingBoundary(t *testing.T) {
	base := api.WatchPlace{Latitude: 30.1234, Longitude: 120.5678}
	sameGrid := api.WatchPlace{Latitude: 30.12341, Longitude: 120.56779} // 差在第 4 位小数，四舍五入后相同
	diffGrid := api.WatchPlace{Latitude: 30.1244, Longitude: 120.5678}   // 差在第 3 位小数，网格不同

	if !SameLocation(base, sameGrid) {
		t.Errorf("expected same grid for 4th-decimal difference: %v vs %v", base, sameGrid)
	}
	if SameLocation(base, diffGrid) {
		t.Errorf("expected different grid for 3rd-decimal difference: %v vs %v", base, diffGrid)
	}
}

func TestMatchesKey_NameAndLocationBothRequired(t *testing.T) {
	existing := api.Watch{Name: "西湖晚霞", Place: api.WatchPlace{Latitude: 30.25, Longitude: 120.15}}

	if !MatchesKey(existing, "西湖晚霞", api.WatchPlace{Latitude: 30.25, Longitude: 120.15}) {
		t.Error("expected match on identical name+location")
	}
	if MatchesKey(existing, "不同名字", api.WatchPlace{Latitude: 30.25, Longitude: 120.15}) {
		t.Error("different name should not match")
	}
	if MatchesKey(existing, "西湖晚霞", api.WatchPlace{Latitude: 31.25, Longitude: 120.15}) {
		t.Error("different location should not match")
	}
}

func TestFieldsEqual_ExactMatch(t *testing.T) {
	existing := api.Watch{
		Template: "trip", TimeMode: "specific", TargetDate: "2026-08-01",
		RepeatRule: "", Scenes: []string{"sunset_glow", "starry_sky"}, VerifyEnabled: true,
	}
	desired := api.CreateWatchRequest{
		Template: "trip", TimeMode: "specific", TargetDate: "2026-08-01",
		RepeatRule: "", Scenes: []string{"starry_sky", "sunset_glow"}, VerifyEnabled: true,
	}
	if !FieldsEqual(existing, desired) {
		t.Error("expected fields to be considered equal (scene order should not matter)")
	}
}

func TestFieldsEqual_DefaultsApplied(t *testing.T) {
	// desired 未填 template/timeMode 时应按后端默认值 trip/unspecified 比较。
	existing := api.Watch{Template: "trip", TimeMode: "unspecified", Scenes: []string{}}
	desired := api.CreateWatchRequest{Scenes: []string{}}
	if !FieldsEqual(existing, desired) {
		t.Error("expected empty template/timeMode in desired to match backend defaults trip/unspecified")
	}
}

func TestFieldsEqual_DateNormalization(t *testing.T) {
	existing := api.Watch{Template: "trip", TimeMode: "specific", TargetDate: "2026-08-01T00:00:00Z", Scenes: nil}
	desired := api.CreateWatchRequest{TimeMode: "specific", TargetDate: "2026-08-01", Scenes: nil}
	if !FieldsEqual(existing, desired) {
		t.Error("expected date-only vs RFC3339 target dates on the same day to be considered equal")
	}
}

func TestFieldsEqual_SceneDifference(t *testing.T) {
	existing := api.Watch{Template: "trip", TimeMode: "unspecified", Scenes: []string{"sunset_glow"}}
	desired := api.CreateWatchRequest{Scenes: []string{"sunrise_glow"}}
	if FieldsEqual(existing, desired) {
		t.Error("expected scene mismatch to break equality")
	}
}

func TestScenesEqual(t *testing.T) {
	if !ScenesEqual([]string{"a", "b"}, []string{"b", "a"}) {
		t.Error("expected unordered equality")
	}
	if ScenesEqual([]string{"a"}, []string{"a", "b"}) {
		t.Error("expected length mismatch to break equality")
	}
}

func TestNormalizeDate(t *testing.T) {
	cases := map[string]string{
		"2026-08-01":           "2026-08-01",
		"2026-08-01T00:00:00Z": "2026-08-01",
		"":                     "",
		"2026-08":              "2026-08",
	}
	for in, want := range cases {
		if got := NormalizeDate(in); got != want {
			t.Errorf("NormalizeDate(%q) = %q, want %q", in, got, want)
		}
	}
}
