package ldap

import (
	"testing"

	"github.com/go-ldap/ldap/v3"

	"hub-server/internal/config"
)

func newEntry(dn string, attrs map[string][]string) *ldap.Entry {
	e := &ldap.Entry{DN: dn}
	for name, vals := range attrs {
		e.Attributes = append(e.Attributes, &ldap.EntryAttribute{Name: name, Values: vals})
	}
	return e
}

func TestParseEntry_GivenNamePresent(t *testing.T) {
	e := newEntry("uid=john,ou=People,dc=test,dc=com", map[string][]string{
		"uid":       {"john"},
		"givenName": {"John Doe"},
		"mail":      {"john@test.com"},
	})
	got, ok := parseEntry(e, "uid", "givenName")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Username != "john" || got.DisplayName != "John Doe" || got.Email != "john@test.com" || got.DN != e.DN {
		t.Fatalf("unexpected parsed entry: %+v", got)
	}
}

// 显示名缺失时回退到 uid（用户名属性）。
func TestParseEntry_DisplayNameFallsBackToUID(t *testing.T) {
	e := newEntry("uid=smith,ou=People,dc=test,dc=com", map[string][]string{
		"uid": {"smith"},
	})
	got, ok := parseEntry(e, "uid", "givenName")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.DisplayName != "smith" {
		t.Fatalf("expected display name fallback to uid 'smith', got %q", got.DisplayName)
	}
}

// 用户名属性缺失时跳过该条目。
func TestParseEntry_MissingUsernameSkipped(t *testing.T) {
	e := newEntry("cn=empty,ou=People,dc=test,dc=com", map[string][]string{
		"givenName": {"Empty"},
	})
	if _, ok := parseEntry(e, "uid", "givenName"); ok {
		t.Fatal("expected ok=false when username attribute is missing")
	}
}

func TestTargets(t *testing.T) {
	cases := []struct {
		targets []string
		manager bool
		web     bool
	}{
		{[]string{"manager"}, true, false},
		{[]string{"manager", "web"}, true, true},
		{[]string{"WEB", "Manager"}, true, true},
		{[]string{}, false, false},
		{[]string{"unknown"}, false, false},
	}
	for _, c := range cases {
		s := &Service{cfg: config.LDAPConfig{SyncTargets: c.targets}}
		m, w := s.targets()
		if m != c.manager || w != c.web {
			t.Errorf("targets=%v: got manager=%v web=%v, want manager=%v web=%v", c.targets, m, w, c.manager, c.web)
		}
	}
}
