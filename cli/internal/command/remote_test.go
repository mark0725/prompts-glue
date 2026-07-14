package command

import (
	"reflect"
	"testing"
	"time"
)

func TestServerToCLIMetaRoundTrip(t *testing.T) {
	now := time.Now().UTC()
	src := PackageMeta{
		Name:        "base-rules",
		Version:     "1.2.3",
		Description: "base rules",
		Status:      "PUBLISHED",
		Category:    "ROLE",
		Type:        "COMPOSITE",
		Expression:  "summary($x){...}",
		Tags:        []string{"a", "b"},
		Dependencies: []DependencyMeta{
			{Name: "dep1", VersionRule: "^1.0.0", ResolvedVersion: "1.2.0", Optional: true, AffectsCompute: true},
		},
		CreatedAt: now.Format(time.RFC3339),
	}
	srv := cliToServerMeta(src)
	if srv.Visibility != "PUBLIC" {
		t.Fatalf("expected visibility PUBLIC, got %q", srv.Visibility)
	}
	got := serverToCLIMeta(srv)
	// CreatedAt 经 time.Time→RFC3339 往返，比较核心字段即可。
	src.CreatedAt = got.CreatedAt
	if !reflect.DeepEqual(src, got) {
		t.Fatalf("round trip mismatch:\nsrc = %+v\ngot = %+v", src, got)
	}
}

func TestServerToCLIDepsEmpty(t *testing.T) {
	if got := serverToCLIDeps(nil); len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}
