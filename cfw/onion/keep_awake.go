package onion

import "os"

// stayAwakeFlag is the file Onion's key monitor looks for before it puts the
// device to sleep: while it exists the sleep timer does nothing and programs
// keep running. Onion removes it itself when a game is launched.
const stayAwakeFlag = "/tmp/stay_awake"

// KeepAwake stops the device sleeping until the returned function is called.
// If something else already asked for it, that is left as it is.
func KeepAwake() (release func()) { return keepAwakeAt(stayAwakeFlag) }

func keepAwakeAt(path string) func() {
	if _, err := os.Stat(path); err == nil {
		return func() {}
	}
	f, err := os.Create(path)
	if err != nil {
		return func() {}
	}
	f.Close()
	return func() { os.Remove(path) }
}
