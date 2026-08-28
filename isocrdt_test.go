// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/go-crdt/crdt"
)

// These property tests are the acceptance gate for the collaborative diagram
// adapter. The structured/crdt core proves its own convergence; what is proved
// HERE is that the adapter — the string-id-to-record mapping and the field
// codecs across all five families — inherits that convergence rather than
// breaking it. Replicas edit concurrently through the widget's own IsoDocument
// interface while an unreliable network delivers batches late, out of order and
// duplicated, and every replica ends byte-identical, with the same entity sets a
// view would render.
//
// The oracle is the byte-equal structured snapshot, reinforced by an equal
// per-family dump. Randomness is seeded, so a failure names the seed and
// reproduces on any platform, js/wasm included. The generic flatten helper is
// shared with the spreadsheet property tests in this package.

// --- randomised edits (id-based, so order-independent) -------------------------

func isoChoose(rng *rand.Rand, opts ...string) string { return opts[rng.IntN(len(opts))] }

func isoColor(rng *rand.Rand) RGBA {
	if rng.IntN(2) == 0 {
		return RGBA{} // the default (theme-inherited) colour, stored as no field
	}
	return RGBA{R: byte(rng.IntN(256)), G: byte(rng.IntN(256)), B: byte(rng.IntN(256)), A: 255}
}

func isoRandNode(rng *rand.Rand) IsoNode {
	return IsoNode{
		ID:    fmt.Sprintf("n%d", rng.IntN(4)),
		X:     rng.IntN(51) - 25,
		Y:     rng.IntN(51) - 25,
		Shape: IsoShape(rng.IntN(3)),
		Icon:  isoChoose(rng, "", "srv", "db"),
		Label: isoChoose(rng, "", "L1", "L2"),
		Color: isoColor(rng),
		Layer: isoChoose(rng, "", "l0", "l1"),
	}
}

func isoRandConn(rng *rand.Rand) IsoConnector {
	return IsoConnector{
		ID:     fmt.Sprintf("c%d", rng.IntN(3)),
		From:   fmt.Sprintf("n%d", rng.IntN(4)),
		To:     fmt.Sprintf("n%d", rng.IntN(4)),
		Label:  isoChoose(rng, "", "E1"),
		Style:  IsoConnectorStyle(rng.IntN(3)),
		Arrow:  IsoArrow(rng.IntN(3)),
		Color:  isoColor(rng),
		Width:  rng.IntN(5),
		Routed: rng.IntN(2) == 0,
		Layer:  isoChoose(rng, "", "l0"),
	}
}

func isoRandZone(rng *rand.Rand) IsoZone {
	return IsoZone{
		ID:    fmt.Sprintf("z%d", rng.IntN(3)),
		X:     rng.IntN(21) - 10,
		Y:     rng.IntN(21) - 10,
		W:     rng.IntN(6),
		H:     rng.IntN(6),
		Color: isoColor(rng),
		Label: isoChoose(rng, "", "Zone"),
		Layer: isoChoose(rng, "", "l0", "l1"),
	}
}

func isoRandText(rng *rand.Rand) IsoText {
	return IsoText{
		ID:    fmt.Sprintf("t%d", rng.IntN(3)),
		X:     rng.IntN(21) - 10,
		Y:     rng.IntN(21) - 10,
		Text:  isoChoose(rng, "", "hi", "note"),
		Color: isoColor(rng),
		Size:  rng.IntN(4),
		Layer: isoChoose(rng, "", "l1"),
	}
}

func isoRandLayer(rng *rand.Rand) IsoLayer {
	return IsoLayer{
		ID:      fmt.Sprintf("l%d", rng.IntN(2)),
		Name:    isoChoose(rng, "", "Base", "Top"),
		Visible: rng.IntN(2) == 0,
		Locked:  rng.IntN(2) == 0,
		Order:   rng.IntN(5),
	}
}

// isoGenEdit picks one random edit and returns it as a closure applied to any
// IsoDocument, so the same edit can drive two stores in lockstep. Every edit
// names an entity by id, never by position, so it is independent of the order a
// store returns its entities — the property that lets the CRDT store (ascending
// by id) and the in-memory store (insertion order) be compared as sets.
func isoGenEdit(rng *rand.Rand) func(IsoDocument) {
	switch rng.IntN(10) {
	case 0:
		n := isoRandNode(rng)
		return func(d IsoDocument) { d.PutNode(n) }
	case 1:
		id := fmt.Sprintf("n%d", rng.IntN(4))
		return func(d IsoDocument) { d.RemoveNode(id) }
	case 2:
		c := isoRandConn(rng)
		return func(d IsoDocument) { d.PutConnector(c) }
	case 3:
		id := fmt.Sprintf("c%d", rng.IntN(3))
		return func(d IsoDocument) { d.RemoveConnector(id) }
	case 4:
		z := isoRandZone(rng)
		return func(d IsoDocument) { d.PutZone(z) }
	case 5:
		id := fmt.Sprintf("z%d", rng.IntN(3))
		return func(d IsoDocument) { d.RemoveZone(id) }
	case 6:
		tx := isoRandText(rng)
		return func(d IsoDocument) { d.PutText(tx) }
	case 7:
		id := fmt.Sprintf("t%d", rng.IntN(3))
		return func(d IsoDocument) { d.RemoveText(id) }
	case 8:
		l := isoRandLayer(rng)
		return func(d IsoDocument) { d.PutLayer(l) }
	default:
		id := fmt.Sprintf("l%d", rng.IntN(2))
		return func(d IsoDocument) { d.RemoveLayer(id) }
	}
}

// --- unreliable transport ------------------------------------------------------

type isoNet struct{ inbox [][]crdt.PartOps }

func newIsoNet(n int) *isoNet { return &isoNet{inbox: make([][]crdt.PartOps, n)} }

func (nw *isoNet) broadcast(from int, batches []crdt.PartOps) {
	for i := range nw.inbox {
		if i != from {
			nw.inbox[i] = append(nw.inbox[i], batches...)
		}
	}
}

func (nw *isoNet) deliver(t *testing.T, rng *rand.Rand, docs []*IsoCRDTDocument, i int) {
	t.Helper()
	queued := nw.inbox[i]
	if len(queued) == 0 {
		return
	}
	n := 1 + rng.IntN(len(queued))
	batch := append([]crdt.PartOps{}, queued[:n]...)
	nw.inbox[i] = queued[n:]
	rng.Shuffle(len(batch), func(a, b int) { batch[a], batch[b] = batch[b], batch[a] })
	if rng.IntN(4) == 0 {
		batch = append(batch, batch[rng.IntN(len(batch))]) // a duplicate must change nothing
	}
	if err := docs[i].Apply(batch...); err != nil {
		t.Fatalf("replica %d: Apply: %v", i, err)
	}
}

func (nw *isoNet) settle(t *testing.T, rng *rand.Rand, docs []*IsoCRDTDocument) {
	t.Helper()
	for i := range docs {
		for len(nw.inbox[i]) > 0 {
			nw.deliver(t, rng, docs, i)
		}
	}
}

func isoRunSession(t *testing.T, seed uint64, replicas int) []*IsoCRDTDocument {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, 0x1503))
	docs := make([]*IsoCRDTDocument, replicas)
	sent := make([]crdt.CompositeVersion, replicas)
	for i := range docs {
		docs[i] = NewIsoCRDTDocument(crdt.SiteID(i + 1))
	}
	nw := newIsoNet(replicas)
	for range 14 {
		for i := range docs {
			for range 1 + rng.IntN(3) {
				isoGenEdit(rng)(docs[i])
			}
			delta := must(docs[i].OpsSince(sent[i]))
			sent[i] = docs[i].Version()
			nw.broadcast(i, delta)
			if rng.IntN(2) == 0 {
				nw.deliver(t, rng, docs, i)
			}
		}
	}
	nw.settle(t, rng, docs)
	return docs
}

// --- per-family dump, order-independent ----------------------------------------

// dumpOf renders a document's whole entity set the way a view sees it, sorted by
// id per family, as a canonical string — so two stores that agree on content
// produce the same dump whatever order they enumerate in, and a nil slice and an
// empty one (the in-memory store returns the former, the CRDT store the latter)
// read the same.
func dumpOf(d IsoDocument) string {
	nodes := d.Nodes()
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	conns := d.Connectors()
	sort.Slice(conns, func(i, j int) bool { return conns[i].ID < conns[j].ID })
	zones := d.Zones()
	sort.Slice(zones, func(i, j int) bool { return zones[i].ID < zones[j].ID })
	texts := d.Texts()
	sort.Slice(texts, func(i, j int) bool { return texts[i].ID < texts[j].ID })
	layers := d.Layers()
	sort.Slice(layers, func(i, j int) bool { return layers[i].ID < layers[j].ID })
	return fmt.Sprintf("N%vC%vZ%vT%vL%v", nodes, conns, zones, texts, layers)
}

// isoConverged reports byte-identical snapshots, nothing pending, equal versions
// and — the adapter-level reinforcement — an equal per-family dump. Returned, not
// asserted, so the control-run can prove it discriminates.
func isoConverged(docs []*IsoCRDTDocument) bool {
	want := docs[0].Snapshot()
	wantDump := dumpOf(docs[0])
	for _, d := range docs {
		if !bytes.Equal(d.Snapshot(), want) {
			return false
		}
		if d.Pending() != 0 {
			return false
		}
		if !d.Version().Equal(docs[0].Version()) {
			return false
		}
		if dumpOf(d) != wantDump {
			return false
		}
	}
	return true
}

func isoAssertConverged(t *testing.T, seed uint64, docs []*IsoCRDTDocument) {
	t.Helper()
	if !isoConverged(docs) {
		for i, d := range docs {
			t.Logf("seed %d: replica %d pending=%d snapshot=%x", seed, i, d.Pending(), d.Snapshot())
		}
		t.Fatalf("seed %d: replicas did not converge", seed)
	}
}

// --- control-run: the oracle must detect divergence ----------------------------

// TestIsoCRDTHarnessDetectsDivergence runs BEFORE the laws are trusted: a single
// withheld operation must read as NOT converged, and supplying it must flip the
// verdict, so a green law below is evidence rather than an accident.
func TestIsoCRDTHarnessDetectsDivergence(t *testing.T) {
	source := isoRunSession(t, 17, 2)
	ops := flatten(must(source[0].OpsSince(nil)))
	if len(ops) < 4 {
		t.Fatalf("fixture too small: %d ops", len(ops))
	}
	full := NewIsoCRDTDocument(1)
	short := NewIsoCRDTDocument(2)
	if err := full.Apply(ops...); err != nil {
		t.Fatal(err)
	}
	if err := short.Apply(ops[:len(ops)-1]...); err != nil { // withhold one
		t.Fatal(err)
	}
	if isoConverged([]*IsoCRDTDocument{full, short}) {
		t.Fatal("oracle failed to notice a withheld operation")
	}
	if err := short.Apply(ops[len(ops)-1]); err != nil {
		t.Fatal(err)
	}
	if !isoConverged([]*IsoCRDTDocument{full, short}) {
		t.Fatal("oracle failed to notice identical state")
	}
}

// --- the four CRDT laws, at the adapter level, across five families ------------

// TestIsoCRDTConvergence: replicas edit all five families concurrently while the
// network delivers late, out of order and with duplicates; once delivery catches
// up every replica must hold byte-identical state and an identical dump.
func TestIsoCRDTConvergence(t *testing.T) {
	for seed := range uint64(200) {
		docs := isoRunSession(t, seed, 2+int(seed%3))
		isoAssertConverged(t, seed, docs)
	}
}

// TestIsoCRDTCommutativity applies one fixed operation set in many orders, one at
// a time, requiring byte-identical state every time — the pending buffer, not the
// batch, doing the reordering.
func TestIsoCRDTCommutativity(t *testing.T) {
	source := isoRunSession(t, 42, 3)
	ops := flatten(must(source[0].OpsSince(nil)))
	if len(ops) < 20 {
		t.Fatalf("only %d operations to permute; fixture too small", len(ops))
	}
	rng := rand.New(rand.NewPCG(7, 7))
	var want []byte
	for trial := range 200 {
		shuffled := append([]crdt.PartOps{}, ops...)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		d := NewIsoCRDTDocument(99)
		for _, b := range shuffled {
			if err := d.Apply(b); err != nil {
				t.Fatalf("trial %d: Apply: %v", trial, err)
			}
		}
		if d.Pending() != 0 {
			t.Fatalf("trial %d: %d operations never became applicable", trial, d.Pending())
		}
		if trial == 0 {
			want = d.Snapshot()
			continue
		}
		if !bytes.Equal(d.Snapshot(), want) {
			t.Fatalf("trial %d: state differs from the first ordering", trial)
		}
	}
}

// TestIsoCRDTIdempotence re-delivers held operations and requires no change.
func TestIsoCRDTIdempotence(t *testing.T) {
	docs := isoRunSession(t, 5, 3)
	isoAssertConverged(t, 5, docs)
	ops := flatten(must(docs[0].OpsSince(nil)))
	before := docs[0].Snapshot()
	for _, b := range ops {
		if err := docs[0].Apply(b); err != nil {
			t.Fatal(err)
		}
	}
	rng := rand.New(rand.NewPCG(1, 2))
	rng.Shuffle(len(ops), func(a, b int) { ops[a], ops[b] = ops[b], ops[a] })
	if err := docs[0].Apply(ops...); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(docs[0].Snapshot(), before) {
		t.Fatal("re-applying operations changed the state")
	}
}

// TestIsoCRDTAssociativity feeds one operation set grouped two different ways;
// the grouping cannot matter.
func TestIsoCRDTAssociativity(t *testing.T) {
	source := isoRunSession(t, 99, 3)
	ops := flatten(must(source[0].OpsSince(nil)))
	rng := rand.New(rand.NewPCG(3, 4))
	rng.Shuffle(len(ops), func(a, b int) { ops[a], ops[b] = ops[b], ops[a] })

	left := NewIsoCRDTDocument(101)  // delivered in a few big groups
	right := NewIsoCRDTDocument(102) // delivered one at a time
	at := 0
	for at < len(ops) {
		step := 1 + rng.IntN(5)
		if at+step > len(ops) {
			step = len(ops) - at
		}
		if err := left.Apply(ops[at : at+step]...); err != nil {
			t.Fatal(err)
		}
		at += step
	}
	for _, b := range ops {
		if err := right.Apply(b); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(left.Snapshot(), right.Snapshot()) {
		t.Fatal("regrouping the same operations produced different state")
	}
}

// TestIsoCRDTLateJoiner: a replica that joins by loading a snapshot is
// indistinguishable from one that replayed the history.
func TestIsoCRDTLateJoiner(t *testing.T) {
	for seed := range uint64(60) {
		docs := isoRunSession(t, seed, 3)
		isoAssertConverged(t, seed, docs)
		joined, err := LoadIsoCRDTDocument(99, docs[0].Snapshot())
		if err != nil {
			t.Fatalf("seed %d: LoadIsoCRDTDocument: %v", seed, err)
		}
		if !bytes.Equal(joined.Snapshot(), docs[0].Snapshot()) {
			t.Fatalf("seed %d: a snapshot did not reload to itself", seed)
		}
		if dumpOf(joined) != dumpOf(docs[0]) {
			t.Fatalf("seed %d: a reloaded snapshot dumped differently", seed)
		}
	}
}

// --- field-wise merge ----------------------------------------------------------

// TestIsoCRDTFieldWiseMerge is the property the per-field register design exists
// for: one replica moves a node while another recolours the SAME node,
// concurrently, and after merge both edits survive — a whole-value store would
// have dropped one.
func TestIsoCRDTFieldWiseMerge(t *testing.T) {
	a := NewIsoCRDTDocument(1)
	b := NewIsoCRDTDocument(2)
	// Both start from the same node.
	a.PutNode(IsoNode{ID: "srv", X: 1, Y: 1, Label: "Server"})
	if err := b.Apply(must(a.OpsSince(nil))...); err != nil {
		t.Fatal(err)
	}
	base := a.Version()

	// Concurrent, disjoint field edits on the same node.
	an, _ := a.Node("srv")
	an.X, an.Y = 9, 4 // a moves it
	a.PutNode(an)
	bn, _ := b.Node("srv")
	bn.Color = RGBA{R: 200, G: 10, B: 10, A: 255} // b recolours it
	b.PutNode(bn)

	// Exchange only the new operations, each way.
	if err := b.Apply(must(a.OpsSince(base))...); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(must(b.OpsSince(base))...); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(a.Snapshot(), b.Snapshot()) {
		t.Fatal("replicas diverged")
	}
	got, ok := a.Node("srv")
	if !ok {
		t.Fatal("node vanished")
	}
	if got.X != 9 || got.Y != 4 {
		t.Fatalf("position lost: got (%d,%d), want (9,4)", got.X, got.Y)
	}
	if (got.Color != RGBA{R: 200, G: 10, B: 10, A: 255}) {
		t.Fatalf("colour lost: got %+v", got.Color)
	}
	if got.Label != "Server" {
		t.Fatalf("untouched label lost: %q", got.Label)
	}
}

// --- fan-out through MVVM ------------------------------------------------------

// TestIsoCRDTFanOut proves the observable seam fires on both a local edit and an
// applied remote batch, and that a remote edit is reflected in the read — so a
// bound IsoDiagram would repaint and show the merged entity.
func TestIsoCRDTFanOut(t *testing.T) {
	a := NewIsoCRDTDocument(1)
	if a.Site() != crdt.SiteID(1) {
		t.Fatalf("Site() = %v, want 1", a.Site())
	}
	local := 0
	unsub := a.Subscribe(func() { local++ })
	before := a.Rev()
	a.PutNode(IsoNode{ID: "n0", X: 3})
	a.PutZone(IsoZone{ID: "z0", W: 2, H: 2})
	if local != 2 {
		t.Fatalf("local edits fired the observable %d times, want 2", local)
	}
	if a.Rev() <= before {
		t.Fatal("revision did not advance on local edits")
	}
	unsub()
	a.PutText(IsoText{ID: "t0"})
	if local != 2 {
		t.Fatal("observer still fired after unsubscribe")
	}

	// A peer applies the whole history; its observable must fire and its read
	// update.
	b := NewIsoCRDTDocument(2)
	remote := 0
	b.Subscribe(func() { remote++ })
	if err := b.Apply(must(a.OpsSince(nil))...); err != nil {
		t.Fatal(err)
	}
	if remote != 1 {
		t.Fatalf("remote apply fired the observable %d times, want 1", remote)
	}
	if n, ok := b.Node("n0"); !ok || n.X != 3 {
		t.Fatalf("remote replica node = %+v, ok=%v", n, ok)
	}
}

// --- retro-compat drop-in ------------------------------------------------------

// TestIsoCRDTDropInMatchesInMemory drives an in-memory IsoDoc and a CRDT document
// through the identical script of IsoDocument calls and requires the same entity
// set from both — the CRDT store is a faithful drop-in. It also confirms the
// default constructor still yields the in-memory store, so a widget built as
// before is unchanged.
func TestIsoCRDTDropInMatchesInMemory(t *testing.T) {
	if _, ok := NewIsoDiagram(nil).Doc().(*IsoDoc); !ok {
		t.Fatal("NewIsoDiagram(nil) no longer defaults to the in-memory IsoDoc")
	}

	mem := NewIsoDoc()
	crd := NewIsoCRDTDocument(1)
	rng := rand.New(rand.NewPCG(2026, 818))
	for range 400 {
		edit := isoGenEdit(rng)
		edit(mem)
		edit(crd)
	}
	if dumpOf(mem) != dumpOf(crd) {
		t.Fatalf("drop-in diverged from in-memory:\n mem=%+v\n crd=%+v", dumpOf(mem), dumpOf(crd))
	}

	// The CRDT document also drives the real widget unchanged: it constructs and
	// reads back its nodes through the widget's own accessor.
	w := NewIsoDiagram(crd)
	if len(w.Doc().Nodes()) != len(crd.Nodes()) {
		t.Fatal("widget on the CRDT document does not see its nodes")
	}
}

// --- field fidelity + edge branches --------------------------------------------

// TestIsoCRDTFieldFidelity round-trips a fully-populated entity of every family
// through the store and back, then reverts a field to its default and confirms
// the override is dropped (the DeleteField path), and that an unknown id reads as
// absent.
func TestIsoCRDTFieldFidelity(t *testing.T) {
	d := NewIsoCRDTDocument(1)

	n := IsoNode{ID: "n1", X: -3, Y: 7, Shape: IsoPyramid, Icon: "db", Label: "DB", Color: RGBA{R: 1, G: 2, B: 3, A: 255}, Layer: "l0"}
	d.PutNode(n)
	if got, ok := d.Node("n1"); !ok || got != n {
		t.Fatalf("node round-trip: got %+v ok=%v, want %+v", got, ok, n)
	}
	c := IsoConnector{ID: "c1", From: "n1", To: "n2", Label: "edge", Style: IsoDashed, Arrow: IsoArrowDouble, Color: RGBA{R: 9, G: 8, B: 7, A: 255}, Width: 3, Routed: true, Layer: "l1"}
	d.PutConnector(c)
	if got := d.Connectors(); len(got) != 1 || got[0] != c {
		t.Fatalf("connector round-trip: %+v", got)
	}
	z := IsoZone{ID: "z1", X: 2, Y: 3, W: 4, H: 5, Color: RGBA{R: 10, G: 20, B: 30, A: 120}, Label: "grp", Layer: "l0"}
	d.PutZone(z)
	if got, ok := d.Zone("z1"); !ok || got != z {
		t.Fatalf("zone round-trip: got %+v ok=%v", got, ok)
	}
	tx := IsoText{ID: "t1", X: -1, Y: -2, Text: "hello", Color: RGBA{R: 5, G: 5, B: 5, A: 255}, Size: 2, Layer: "l1"}
	d.PutText(tx)
	if got, ok := d.Text("t1"); !ok || got != tx {
		t.Fatalf("text round-trip: got %+v ok=%v", got, ok)
	}
	l := IsoLayer{ID: "l0", Name: "Base", Visible: true, Locked: true, Order: 3}
	d.PutLayer(l)
	if got, ok := d.Layer("l0"); !ok || got != l {
		t.Fatalf("layer round-trip: got %+v ok=%v", got, ok)
	}

	// Revert a non-default field to its default on an existing node: the override
	// must be dropped (DeleteField), leaving the field at its zero value.
	n.Label = ""
	n.X = 0
	d.PutNode(n)
	got, _ := d.Node("n1")
	if got.Label != "" || got.X != 0 {
		t.Fatalf("field revert not applied: %+v", got)
	}
	if fields := dumpString(d); fields == "" {
		t.Fatal("unexpected empty document")
	}

	// An unknown id in every getter reads as absent.
	if _, ok := d.Node("nope"); ok {
		t.Fatal("unknown node reported present")
	}
	if _, ok := d.Zone("nope"); ok {
		t.Fatal("unknown zone reported present")
	}
	if _, ok := d.Text("nope"); ok {
		t.Fatal("unknown text reported present")
	}
	if _, ok := d.Layer("nope"); ok {
		t.Fatal("unknown layer reported present")
	}
}

// dumpString is a tiny helper that forces a read of every family, used to assert
// a non-empty document after edits.
func dumpString(d IsoDocument) string {
	return fmt.Sprintf("%d/%d/%d/%d/%d", len(d.Nodes()), len(d.Connectors()), len(d.Zones()), len(d.Texts()), len(d.Layers()))
}

// TestIsoCRDTRemoveNodeCascades proves RemoveNode drops every connector touching
// the node — the source, the target and none-of-the-above cases — and that
// removing an absent entity is a silent no-op.
func TestIsoCRDTRemoveNodeCascades(t *testing.T) {
	d := NewIsoCRDTDocument(1)
	d.PutNode(IsoNode{ID: "a"})
	d.PutNode(IsoNode{ID: "b"})
	d.PutNode(IsoNode{ID: "c"})
	d.PutConnector(IsoConnector{ID: "ab", From: "a", To: "b"}) // touches a (source)
	d.PutConnector(IsoConnector{ID: "ba", From: "b", To: "a"}) // touches a (target)
	d.PutConnector(IsoConnector{ID: "bc", From: "b", To: "c"}) // does not touch a

	d.RemoveNode("a")
	if _, ok := d.Node("a"); ok {
		t.Fatal("node a still present")
	}
	conns := d.Connectors()
	if len(conns) != 1 || conns[0].ID != "bc" {
		t.Fatalf("cascade wrong: remaining %+v, want only bc", conns)
	}
	// Removing entities that do not exist is a no-op across families.
	d.RemoveNode("ghost")
	d.RemoveConnector("ghost")
	d.RemoveZone("ghost")
	d.RemoveText("ghost")
	d.RemoveLayer("ghost")
	if len(d.Connectors()) != 1 {
		t.Fatal("a no-op remove changed the document")
	}
}

// TestIsoCRDTErrors exercises the error paths: a malformed snapshot and an
// invalid applied batch, which must change nothing.
func TestIsoCRDTErrors(t *testing.T) {
	if _, err := LoadIsoCRDTDocument(1, []byte("not a snapshot")); err == nil {
		t.Fatal("LoadIsoCRDTDocument on garbage did not error")
	}
	d := NewIsoCRDTDocument(1)
	d.PutNode(IsoNode{ID: "n0"})
	before := d.Rev()
	if err := d.Apply(crdt.PartOps{}); err == nil {
		t.Fatal("Apply of an invalid batch did not error")
	}
	if d.Rev() != before {
		t.Fatal("a failed Apply still ticked the revision")
	}
}
