package common

import (
	"fmt"
	"sync"
)

const (
	//CONTEXT_APP_PANEL_OPEN = "app.panel.open"
	CONTEXT_LONG_TASK_LIST = "long.task.list"
	CONTEXT_APP_MAIN       = "app.main.context"
	CONTEXT_SAVE_TEMPLATE  = "app.save.template"
)

var uiContext *UIContext = nil

func init() {
	uiContext = &UIContext{
		lock:       sync.RWMutex{},
		contextMap: make(map[string]*ContextData),
	}

	uiContext.register(CONTEXT_LONG_TASK_LIST, &LongTasksContext{
		queue: make([]*LongTask, 0),
	}, true)

	uiContext.register(CONTEXT_APP_MAIN, nil, true)

	uiContext.register(CONTEXT_SAVE_TEMPLATE, false, true)

}

func GetUIContext() *UIContext {
	return uiContext
}

type ContextData struct {
	value          any
	extra          any
	requireRefresh bool
}

// The ui context is suitable for ui state changing actions
// such as checkbox and selections by clicking
// for one-time actions, like click button to do something once (like loading)
// in-place processing seems  better
// as if you are using context you need clear the context value after
// the action has been performed. Otherwise it always perform the same action
// on each Frame event (ui layout)
// Using this context can be convenient for cross component ui updates
// like you click a button in one panel and other panel can get it
// from the context and update their UIs (no need to access the panels button event)
type UIContext struct {
	lock       sync.RWMutex
	contextMap map[string]*ContextData
}

func (uictx *UIContext) register(key string, defaultValue any, requireRefresh bool) error {
	uictx.lock.Lock()
	defer uictx.lock.Unlock()
	if _, ok := uictx.contextMap[key]; ok {
		return fmt.Errorf("context alread registered %v", key)
	}
	uictx.contextMap[key] = &ContextData{
		value:          defaultValue,
		extra:          nil,
		requireRefresh: requireRefresh,
	}
	return nil
}

func (uictx *UIContext) getValue(key string) (any, any) {
	uictx.lock.RLock()
	defer uictx.lock.RUnlock()
	if ctxData, ok := uictx.contextMap[key]; ok {
		return ctxData.value, ctxData.extra
	}
	return nil, nil
}

func (uictx *UIContext) setValue(key string, value any, extra any) error {
	uictx.lock.Lock()
	defer uictx.lock.Unlock()
	if ctxData, ok := uictx.contextMap[key]; ok {
		ctxData.value = value
		ctxData.extra = extra
		if ctxData.requireRefresh {
			if win := GetAppWindow(); win != nil {
				win.Invalidate()
			}
		}
		return nil
	}
	return fmt.Errorf("no such context %v", key)
}

// Helpers

// call it before layout
func RegisterContext(key string, defaultValue any, requireRefresh bool) error {
	return uiContext.register(key, defaultValue, requireRefresh)
}

func SetContextBool(key string, value bool, extra any) error {
	return uiContext.setValue(key, value, extra)
}

func GetContextBool(key string) (bool, any, error) {
	return uiContext.getBool(key)
}

func FlipContextBool(key string) (bool, error) {
	return uiContext.flipBool(key)
}

func (uictx *UIContext) getBool(key string) (bool, any, error) {
	val, ex := uictx.getValue(key)
	if value, ok := val.(bool); ok {
		return value, ex, nil
	}
	return false, nil, fmt.Errorf("invalid type %T", val)
}

// Returns previous value
func (uictx *UIContext) flipBool(key string) (bool, error) {
	uictx.lock.Lock()
	defer uictx.lock.Unlock()
	if data, ok := uictx.contextMap[key]; ok {
		if bv, ok := data.value.(bool); ok {
			data.value = !bv
			return bv, nil
		}
		return false, fmt.Errorf("type mismatch %T", data.value)
	}
	return false, fmt.Errorf("no such context %v", key)
}

func SetContextData(key string, data any, extra any) {
	uiContext.setValue(key, data, extra)
}

func GetContextData(key string) (any, any) {
	return uiContext.getValue(key)
}

type LongTask struct {
	context  *LongTasksContext
	Name     string
	Status   string
	Progress float32
	Step     float32
	Run      func()
	isDone   bool
}

func (lt *LongTask) IsDone() bool {
	return lt.isDone
}

func (lt *LongTask) Failed(err error) {
	lt.Status = err.Error()
	lt.Done()
}

func (lt *LongTask) Done() {
	lt.context.RemoveTask(lt.Name)
	lt.isDone = true
}

func (lt *LongTask) Start() {
	go lt.Run()
}

func (lt *LongTask) Update(status string) {
	lt.Progress += lt.Step
	if lt.Progress >= 1.0 {
		lt.Done()
	} else {
		lt.Status = status
	}
	GetAppWindow().Invalidate()
}

type LongTasksContext struct {
	longTaskLock sync.RWMutex
	queue        []*LongTask
}

func (ltc *LongTasksContext) RemoveTask(name string) {
	ltc.longTaskLock.Lock()
	defer ltc.longTaskLock.Unlock()

	for i, task := range ltc.queue {
		if task.Name == name {
			ltc.queue = append(ltc.queue[:i], ltc.queue[i+1:]...)
			break
		}
	}
}

func (ltc *LongTasksContext) AddTask(taskName string) *LongTask {
	ltc.longTaskLock.Lock()
	defer ltc.longTaskLock.Unlock()

	newTask := &LongTask{
		context:  ltc,
		Name:     taskName,
		Progress: 0.0,
	}

	ltc.queue = append(ltc.queue, newTask)
	return newTask
}

func (ltc *LongTasksContext) GetFirstTask() *LongTask {
	ltc.longTaskLock.RLock()
	defer ltc.longTaskLock.RUnlock()

	if len(ltc.queue) > 0 {
		return ltc.queue[0]
	}
	return nil
}
