//go:build darwin && !cgo

package desktop

func NewTrayAdapter(map[TrayState][]byte, map[TrayState]string) TrayAdapter {
	return noopTrayAdapter{}
}
