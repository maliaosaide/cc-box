// 快照管理
// 快照结构定义、序列化、链式存储
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Snapshot 快照结构
type Snapshot struct {
	ID        string             `json:"id"`
	Parent    string             `json:"parent,omitempty"`
	Timestamp time.Time          `json:"timestamp"`
	Device    string             `json:"device"`
	Message   string             `json:"message"`
	Files     map[string]FileEntry `json:"files"`
	Binary    map[string]map[string]string `json:"binary,omitempty"`
}

// FileEntry 文件条目
type FileEntry struct {
	Hash     string    `json:"hash"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// ChangeType 变更类型
type ChangeType int

const (
	Added   ChangeType = iota // 新增
	Modified                  // 修改
	Deleted                   // 删除
)

// Change 文件变更
type Change struct {
	Path    string
	Type    ChangeType
	OldHash string // 修改/删除时旧哈希
	NewHash string // 新增/修改时新哈希
	OldSize int64
	NewSize int64
}

// New 创建新快照
func New(parent, device, message string) *Snapshot {
	now := time.Now().UTC()
	h := sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%d", device, now.Format(time.RFC3339Nano), now.UnixNano())))
	id := "snap_" + hex.EncodeToString(h[:])[:8]

	return &Snapshot{
		ID:        id,
		Parent:    parent,
		Timestamp: now,
		Device:    device,
		Message:   message,
		Files:     make(map[string]FileEntry),
		Binary:    make(map[string]map[string]string),
	}
}

// Serialize 序列化快照为 JSON
func (s *Snapshot) Serialize() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// Deserialize 从 JSON 反序列化快照
func Deserialize(data []byte) (*Snapshot, error) {
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("反序列化快照失败: %w", err)
	}
	if snap.Files == nil {
		snap.Files = make(map[string]FileEntry)
	}
	return &snap, nil
}

// Diff 计算两个快照之间的差异
// old 可以为 nil（首次快照时所有文件都是 Added）
func (old *Snapshot) Diff(new_ *Snapshot) []Change {
	var changes []Change

	// 找出新增和修改
	for path, newEntry := range new_.Files {
		if oldEntry, exists := old.Files[path]; exists {
			if oldEntry.Hash != newEntry.Hash {
				changes = append(changes, Change{
					Path:    path,
					Type:    Modified,
					OldHash: oldEntry.Hash,
					NewHash: newEntry.Hash,
					OldSize: oldEntry.Size,
					NewSize: newEntry.Size,
				})
			}
		} else {
			changes = append(changes, Change{
				Path:    path,
				Type:    Added,
				NewHash: newEntry.Hash,
				NewSize: newEntry.Size,
			})
		}
	}

	// 找出删除
	for path, oldEntry := range old.Files {
		if _, exists := new_.Files[path]; !exists {
			changes = append(changes, Change{
				Path:    path,
				Type:    Deleted,
				OldHash: oldEntry.Hash,
				OldSize: oldEntry.Size,
			})
		}
	}

	return changes
}

// FindCommonAncestor 查找两个快照链的共同祖先
// 遍历本地和远程的快照链，找到第一个公共快照 ID
func FindCommonAncestor(localChain, remoteChain []string, loader func(id string) (*Snapshot, error)) (string, error) {
	localSet := make(map[string]bool)
	for _, id := range localChain {
		localSet[id] = true
	}

	for _, id := range remoteChain {
		if localSet[id] {
			return id, nil
		}
	}

	return "", ErrNoCommonAncestor
}

// ErrNoCommonAncestor 找不到共同祖先
var ErrNoCommonAncestor = fmt.Errorf("找不到共同祖先快照")

// BuildChain 沿 parent 链回溯构建快照 ID 列表
func BuildChain(headID string, loader func(id string) (*Snapshot, error), limit int) ([]string, error) {
	var chain []string
	current := headID

	for i := 0; i < limit && current != ""; i++ {
		chain = append(chain, current)
		snap, err := loader(current)
		if err != nil {
			break
		}
		current = snap.Parent
	}

	return chain, nil
}
