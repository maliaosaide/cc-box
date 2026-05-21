package desktop

type TrayState string

const (
	TraySynced   TrayState = "synced"
	TrayPending  TrayState = "pending"
	TrayConflict TrayState = "conflict"
	TraySyncing  TrayState = "syncing"
)

type TrayActions struct {
	OnPush func()
	OnPull func()
	OnSync func()
	OnOpen func()
	OnQuit func()
}

type TrayAdapter interface {
	Start(actions TrayActions) error
	Stop()
	SetState(state TrayState)
	IsReady() bool
}

type noopTrayAdapter struct{}

func (noopTrayAdapter) Start(TrayActions) error { return nil }

func (noopTrayAdapter) Stop() {}

func (noopTrayAdapter) SetState(TrayState) {}

func (noopTrayAdapter) IsReady() bool { return false }
