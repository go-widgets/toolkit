package toolkit

import "testing"

// TestAFallbackEntryTakesKeysThroughItsNative is the keyboard half of the
// fallback contract: a Native that no host claims is the real control, so a
// person must be able to click its Entry and type into it.
//
// ⛔ It was not so. Native was not a focusEnumerator, the focus walk stopped at
// it, and routeFocusKey never found the Entry it had just focused: the click
// moved focus, the characters went nowhere.
func TestAFallbackEntryTakesKeysThroughItsNative(t *testing.T) {
	entry := NewEntry("ab")
	n := NewNativeEntry("")
	n.Fallback = entry
	box := NewVBox()
	box.AddFixed(NewFormField("Name", n), 80)
	box.SetBounds(Rect{W: 300, H: 200})

	b := entry.Bounds()
	box.OnEvent(Event{Kind: EventClick, X: b.X + 2, Y: b.Y + 2})
	if !entry.Focused() {
		t.Fatal("the click did not focus the fallback entry")
	}
	box.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	box.OnEvent(Event{Kind: EventChar, Code: "c"})
	if got := entry.Text().Get(); got != "abc" {
		t.Fatalf("the fallback entry holds %q after typing, want %q", got, "abc")
	}

	// Once a host claims the region its own control has the keys, so the
	// walk must not hand them to a fallback nobody can see.
	n.Claimed().Set(true)
	box.OnEvent(Event{Kind: EventChar, Code: "d"})
	if got := entry.Text().Get(); got != "abc" {
		t.Fatalf("a claimed region's hidden fallback took a key: %q", got)
	}
}
