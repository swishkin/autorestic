package internal

import "testing"

func TestGetType(t *testing.T) {

	t.Run("TypeLocal", func(t *testing.T) {
		l := Location{
			Type: "local",
		}
		result, err := l.getType()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		assertEqual(t, result, TypeLocal)
	})

	t.Run("TypeVolume", func(t *testing.T) {
		l := Location{
			Type: "volume",
		}
		result, err := l.getType()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		assertEqual(t, result, TypeVolume)
	})

	t.Run("Empty type", func(t *testing.T) {
		l := Location{
			Type: "",
		}
		result, err := l.getType()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		assertEqual(t, result, TypeLocal)
	})

	t.Run("Invalid type", func(t *testing.T) {
		l := Location{
			Type: "foo",
		}
		_, err := l.getType()
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestBuildTag(t *testing.T) {
	result := buildTag("foo", "bar")
	expected := "ar:foo:bar"
	assertEqual(t, result, expected)
}

func TestGetLocationTags(t *testing.T) {
	l := Location{
		name: "foo",
	}
	result := l.getLocationTags()
	expected := "ar:location:foo"
	assertEqual(t, result, expected)
}

func TestHasBackend(t *testing.T) {
	t.Run("backend present", func(t *testing.T) {
		l := Location{
			name: "foo",
			To:   []string{"foo", "bar"},
		}
		result := l.hasBackend("foo")
		assertEqual(t, result, true)
	})

	t.Run("backend absent", func(t *testing.T) {
		l := Location{
			name: "foo",
			To:   []string{"bar", "baz"},
		}
		result := l.hasBackend("foo")
		assertEqual(t, result, false)
	})
}

func TestBuildRestoreCommand(t *testing.T) {
	l := Location{
		name: "foo",
	}
	result := buildRestoreCommand(l, "to", "snapshot", []string{"options"})
	expected := []string{"restore", "--target", "to", "--tag", "ar:location:foo", "snapshot", "options"}
	assertSliceEqual(t, result, expected)
}

func TestIsLockError(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		output string
		want   bool
	}{
		{"exit code 11", 11, "", true},
		{"already locked", 1, "repo already locked, waiting up to 0s for the lock", true},
		{"invalid lock", 1, "Fatal: invalid lock file: LoadRaw(<lock/ffffffffff>)", true},
		{"unrelated error", 1, "Fatal: unable to open repository", false},
		{"success", 0, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertEqual(t, isLockError(tt.code, tt.output), tt.want)
		})
	}
}

func TestBackendsToForget(t *testing.T) {
	t.Run("copy targets are included", func(t *testing.T) {
		l := Location{
			To: []string{"a", "b"},
			CopyOption: LocationCopy{
				"a": {"c"},
			},
		}
		assertSliceEqual(t, l.backendsToForget(), []string{"a", "b", "c"})
	})

	t.Run("duplicates are removed", func(t *testing.T) {
		l := Location{
			To: []string{"a", "a"},
			CopyOption: LocationCopy{
				"a": {"a", "c", "c"},
			},
		}
		assertSliceEqual(t, l.backendsToForget(), []string{"a", "c"})
	})

	t.Run("does not mutate the location", func(t *testing.T) {
		l := Location{
			To: []string{"a"},
			CopyOption: LocationCopy{
				"a": {"c"},
			},
		}
		l.backendsToForget()
		assertSliceEqual(t, l.To, []string{"a"})
	})
}
