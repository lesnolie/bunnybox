package profile

import "testing"

func TestPreferencesPersist(t *testing.T) {
	root := t.TempDir()
	s := openTest(t, root)
	if s.PreferenceString(testContext, "language", "zh-Hans") != "zh-Hans" || !s.PreferenceBool(testContext, "transparency", true) {
		t.Fatal("fallbacks")
	}
	if e := s.SetPreference(testContext, "language", "en"); e != nil {
		t.Fatal(e)
	}
	if e := s.SetPreference(testContext, "transparency", false); e != nil {
		t.Fatal(e)
	}
	if s.SetPreference(testContext, "selected_profile_id", int64(1)) == nil {
		t.Fatal("selection must go through Select")
	}
	s.Close()
	s = openTest(t, root)
	if s.PreferenceString(testContext, "language", "zh-Hans") != "en" || s.PreferenceBool(testContext, "transparency", true) {
		t.Fatal("not persisted")
	}
}
