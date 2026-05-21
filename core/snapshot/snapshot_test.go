// 快照管理测试
package snapshot

import (
	"testing"
	"time"
)

func TestNewSnapshot(t *testing.T) {
	snap := New("snap_parent", "device1", "test message")
	if snap.Parent != "snap_parent" {
		t.Errorf("Parent = %s, want snap_parent", snap.Parent)
	}
	if snap.Device != "device1" {
		t.Errorf("Device = %s, want device1", snap.Device)
	}
	if snap.Message != "test message" {
		t.Errorf("Message = %s, want test message", snap.Message)
	}
	if snap.ID == "" {
		t.Error("ID should not be empty")
	}
	if snap.ID[:5] != "snap_" {
		t.Errorf("ID should start with snap_, got %s", snap.ID)
	}
	if snap.Files == nil {
		t.Error("Files should be initialized")
	}
}

func TestSerializeDeserialize(t *testing.T) {
	snap := New("snap_parent", "device1", "test")
	snap.Files["settings.json"] = FileEntry{
		Hash:     "sha256:abc123",
		Size:     1024,
		Modified: time.Now().UTC(),
	}

	data, err := snap.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.ID != snap.ID {
		t.Errorf("ID mismatch: %s vs %s", restored.ID, snap.ID)
	}
	if restored.Parent != snap.Parent {
		t.Errorf("Parent mismatch")
	}
	if len(restored.Files) != 1 {
		t.Errorf("Files count = %d, want 1", len(restored.Files))
	}
	if restored.Files["settings.json"].Hash != "sha256:abc123" {
		t.Error("File entry hash mismatch")
	}
}

func TestDiffAdded(t *testing.T) {
	old := New("", "d1", "old")
	old.Files["a.txt"] = FileEntry{Hash: "hash_a"}

	new_ := New("", "d1", "new")
	new_.Files["a.txt"] = FileEntry{Hash: "hash_a"}
	new_.Files["b.txt"] = FileEntry{Hash: "hash_b"}

	changes := old.Diff(new_)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != Added {
		t.Errorf("change type = %d, want Added", changes[0].Type)
	}
	if changes[0].Path != "b.txt" {
		t.Errorf("change path = %s, want b.txt", changes[0].Path)
	}
}

func TestDiffModified(t *testing.T) {
	old := New("", "d1", "old")
	old.Files["a.txt"] = FileEntry{Hash: "hash_v1"}

	new_ := New("", "d1", "new")
	new_.Files["a.txt"] = FileEntry{Hash: "hash_v2"}

	changes := old.Diff(new_)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != Modified {
		t.Errorf("change type = %d, want Modified", changes[0].Type)
	}
}

func TestDiffDeleted(t *testing.T) {
	old := New("", "d1", "old")
	old.Files["a.txt"] = FileEntry{Hash: "hash_a"}
	old.Files["b.txt"] = FileEntry{Hash: "hash_b"}

	new_ := New("", "d1", "new")
	new_.Files["a.txt"] = FileEntry{Hash: "hash_a"}

	changes := old.Diff(new_)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != Deleted {
		t.Errorf("change type = %d, want Deleted", changes[0].Type)
	}
	if changes[0].Path != "b.txt" {
		t.Errorf("change path = %s, want b.txt", changes[0].Path)
	}
}

func TestDiffNoChanges(t *testing.T) {
	old := New("", "d1", "old")
	old.Files["a.txt"] = FileEntry{Hash: "hash_a"}

	new_ := New("", "d1", "new")
	new_.Files["a.txt"] = FileEntry{Hash: "hash_a"}

	changes := old.Diff(new_)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
}

func TestDiffAllTypes(t *testing.T) {
	old := New("", "d1", "old")
	old.Files["a.txt"] = FileEntry{Hash: "hash_a", Size: 100}
	old.Files["b.txt"] = FileEntry{Hash: "hash_b", Size: 200}
	old.Files["c.txt"] = FileEntry{Hash: "hash_c", Size: 300}

	new_ := New("", "d1", "new")
	new_.Files["a.txt"] = FileEntry{Hash: "hash_a", Size: 100}  // unchanged
	new_.Files["b.txt"] = FileEntry{Hash: "hash_b2", Size: 250} // modified
	new_.Files["d.txt"] = FileEntry{Hash: "hash_d", Size: 400}  // added
	// c.txt deleted

	changes := old.Diff(new_)
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}

	typeCounts := make(map[ChangeType]int)
	for _, c := range changes {
		typeCounts[c.Type]++
	}
	if typeCounts[Modified] != 1 || typeCounts[Deleted] != 1 || typeCounts[Added] != 1 {
		t.Errorf("unexpected change counts: %v", typeCounts)
	}
}
