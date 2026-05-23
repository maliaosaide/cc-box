// 异步操作管理
// goroutine + EventsEmit 进度推送 + 取消支持
package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxStoredOpResults = 64

var (
	opCounter     atomic.Int64
	opCancels     = make(map[int64]context.CancelFunc)
	opOperations  = make(map[int64]string)
	opCancelMu    sync.Mutex
	opResults     = make(map[int64]error)
	opResultOrder []int64

	trayOpMu      sync.Mutex
	trayActiveOps int
	trayHadError  bool
)

// ProgressEvent 进度事件
type ProgressEvent struct {
	OpID      int64   `json:"opId"`
	Operation string  `json:"operation"`
	Current   int64   `json:"current"`
	Total     int64   `json:"total"`
	Percent   float64 `json:"percent"`
	Part      int     `json:"part"`
	TotalPart int     `json:"totalPart"`
	Message   string  `json:"message"`
}

// OpResult 操作结果事件
type OpResult struct {
	OpID      int64  `json:"opId"`
	Operation string `json:"operation"`
	Error     string `json:"error,omitempty"`
	Status    string `json:"status"` // "success" | "error"
}

type DataChangedEvent struct {
	Domain string `json:"domain"`
	Source string `json:"source"`
}

// StartAsync 启动异步操作，返回 opId
func (a *App) StartAsync(operation string, fn func(ctx context.Context, opID int64) error) int64 {
	opID := opCounter.Add(1)
	ctx, cancel := context.WithCancel(context.Background())

	opCancelMu.Lock()
	opCancels[opID] = cancel
	opOperations[opID] = operation
	opCancelMu.Unlock()

	go func() {
		var err error
		trackTray := operationTracksTray(operation)
		if trackTray {
			beginTrayOperation()
		}
		defer func() {
			opCancelMu.Lock()
			delete(opCancels, opID)
			delete(opOperations, opID)
			recordOpResultLocked(opID, err)
			opCancelMu.Unlock()
		}()

		err = fn(ctx, opID)
		if trackTray {
			finishTrayOperation(err)
		}

		result := OpResult{OpID: opID, Operation: operation, Status: "success"}
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		a.eventsEmit("op:complete", result)
	}()

	return opID
}

// CancelOperation 取消异步操作
func (a *App) CancelOperation(opID int64) {
	opCancelMu.Lock()
	cancel, ok := opCancels[opID]
	operation := opOperations[opID]
	opCancelMu.Unlock()

	if ok {
		cancel()
		a.eventsEmit("op:cancelled", OpResult{OpID: opID, Operation: operation, Status: "error", Error: "已取消"})
	}
}

func recordOpResultLocked(opID int64, err error) {
	if _, exists := opResults[opID]; !exists {
		opResultOrder = append(opResultOrder, opID)
	}
	opResults[opID] = err
	for len(opResultOrder) > maxStoredOpResults {
		oldest := opResultOrder[0]
		opResultOrder = opResultOrder[1:]
		delete(opResults, oldest)
	}
}

func takeOpResult(opID int64) (error, bool) {
	opCancelMu.Lock()
	defer opCancelMu.Unlock()
	err, ok := opResults[opID]
	if !ok {
		return nil, false
	}
	delete(opResults, opID)
	removeOpResultOrderLocked(opID)
	return err, true
}

func removeOpResultOrderLocked(opID int64) {
	for i, id := range opResultOrder {
		if id == opID {
			opResultOrder = append(opResultOrder[:i], opResultOrder[i+1:]...)
			return
		}
	}
}

func operationTracksTray(operation string) bool {
	return strings.HasPrefix(operation, "quick-") || strings.HasPrefix(operation, "bulk-") || operation == "repair-remote"
}

func beginTrayOperation() {
	trayOpMu.Lock()
	wasIdle := trayActiveOps == 0
	if wasIdle {
		trayHadError = false
	}
	trayActiveOps++
	trayOpMu.Unlock()

	if wasIdle {
		UpdateTrayState(TraySyncing)
	}
}

func finishTrayOperation(err error) {
	trayOpMu.Lock()
	if err != nil {
		trayHadError = true
	}
	if trayActiveOps > 0 {
		trayActiveOps--
	}
	if trayActiveOps != 0 {
		trayOpMu.Unlock()
		return
	}
	state := TraySynced
	if trayHadError {
		state = TrayConflict
	}
	trayHadError = false
	trayOpMu.Unlock()

	UpdateTrayState(state)
}

// emitProgress 推送进度事件
func (a *App) eventsEmit(event string, payload interface{}) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, event, payload)
}

func (a *App) emitDataChanged(domain, source string) {
	a.eventsEmit("data:changed", DataChangedEvent{Domain: domain, Source: source})
}

func (a *App) emitProgress(opID int64, operation string, current, total int64, part, totalPart int, msg string) {
	var pct float64
	if total > 0 {
		pct = float64(current) / float64(total) * 100
	}

	a.eventsEmit("op:progress", ProgressEvent{
		OpID:      opID,
		Operation: operation,
		Current:   current,
		Total:     total,
		Percent:   pct,
		Part:      part,
		TotalPart: totalPart,
		Message:   msg,
	})
}

// ProgressCallback 返回适配 internal 包进度签名的回调
func (a *App) progressCallback(opID int64, operation string) func(total, uploaded int64, part, totalParts int) {
	return func(total, uploaded int64, part, totalParts int) {
		msg := fmt.Sprintf("%d/%d 分块", part, totalParts)
		a.emitProgress(opID, operation, uploaded, total, part, totalParts, msg)
	}
}
