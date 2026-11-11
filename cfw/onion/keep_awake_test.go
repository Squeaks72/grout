package onion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeepAwake_FlagExistsUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stay_awake")

	release := keepAwakeAt(path)
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the flag was not created")
	}
	release()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the flag was left behind")
	}
}

// A flag someone else set is theirs to remove.
func TestKeepAwake_LeavesAFlagSomeoneElseSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stay_awake")
	os.WriteFile(path, nil, 0644)

	keepAwakeAt(path)()

	if _, err := os.Stat(path); err != nil {
		t.Error("released a flag it did not create")
	}
}

func TestKeepAwake_AnUnwritableFlagIsNotAnError(t *testing.T) {
	keepAwakeAt(filepath.Join(t.TempDir(), "no", "such", "dir", "stay_awake"))()
}

// An inner hold finding the flag already set must not end the outer one.
func TestKeepAwake_InnerHoldDoesNotEndTheOuter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stay_awake")
	outer := keepAwakeAt(path)
	inner := keepAwakeAt(path)

	inner()

	if _, err := os.Stat(path); err != nil {
		t.Fatal("the inner hold removed the outer one's flag")
	}
	outer()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the flag outlived its owner")
	}
}
