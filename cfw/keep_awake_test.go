package cfw

import "testing"

func TestKeepAwake_ReleasesOnceNoMatterHowOftenItIsCalled(t *testing.T) {
	var asked, released int
	f := &Firmware{keepAwake: func() func() {
		asked++
		return func() { released++ }
	}}

	release := keepAwake(f)
	release()
	release()

	if asked != 1 || released != 1 {
		t.Errorf("asked %d, released %d; want 1 and 1", asked, released)
	}
}

func TestKeepAwake_NoSupportIsHarmless(t *testing.T) {
	keepAwake(&Firmware{})()
	keepAwake(nil)()
}
