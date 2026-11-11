package cfw

// KeepAwake stops the device sleeping until the returned function is called,
// for a long transfer the player is only watching. It does nothing on firmware
// with no way to ask, and the function is safe to call more than once.
//
// A hold that finds the device already being kept awake leaves that as it is,
// so an inner hold never ends an outer one.
func KeepAwake() (release func()) { return keepAwake(ActiveFirmware()) }

func keepAwake(f *Firmware) func() {
	if f == nil || f.keepAwake == nil {
		return func() {}
	}
	release := f.keepAwake()
	var done bool
	return func() {
		if !done {
			done = true
			release()
		}
	}
}
