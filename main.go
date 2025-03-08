package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// State represents the possible states of a VM
type State string

const (
	Pending      State = "PENDING"
	Provisioning State = "PROVISIONING"
	Running      State = "RUNNING"
	Paused       State = "PAUSED"
	Stopping     State = "STOPPING"
	Stopped      State = "STOPPED"
	Failed       State = "FAILED"
	Terminated   State = "TERMINATED"
)

// Event represents possible events that can trigger state transitions
type Event string

const (
	StartProvision   Event = "START_PROVISION"
	ProvisionSuccess Event = "PROVISION_SUCCESS"
	ProvisionFail    Event = "PROVISION_FAIL"
	StartVM          Event = "START_VM"
	PauseVM          Event = "PAUSE_VM"
	ResumeVM         Event = "RESUME_VM"
	StopVM           Event = "STOP_VM"
	TerminateVM      Event = "TERMINATE_VM"
	Fail             Event = "FAIL"
)

// Task represents a VM task
type Task struct {
	ID             string
	CurrentState   State
	transitionChan chan Event
	doneChan       chan struct{}
	stateMu        sync.RWMutex
}

// NewTask creates a new VM task
func NewTask(id string) *Task {
	return &Task{
		ID:             id,
		CurrentState:   Pending,
		transitionChan: make(chan Event, 10),
		doneChan:       make(chan struct{}),
	}
}

// GetState returns the current state of the VM
func (t *Task) GetState() State {
	t.stateMu.RLock()
	defer t.stateMu.RUnlock()
	return t.CurrentState
}

// setState updates the state of the VM
func (t *Task) setState(newState State) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.CurrentState = newState
}

// SendEvent sends an event to the task's state machine
func (t *Task) SendEvent(event Event) error {
	select {
	case t.transitionChan <- event:
		return nil
	case <-t.doneChan:
		return fmt.Errorf("task %s is terminated", t.ID)
	}
}

// Start starts the state machine for this task
func (t *Task) Start() {
	go t.runStateMachine()
}

// Stop stops the state machine for this task
func (t *Task) Stop() {
	close(t.doneChan)
}

// runStateMachine is the main goroutine that manages state transitions
func (t *Task) runStateMachine() {
	fmt.Printf("Task %s: Starting state machine, initial state: %s\n", t.ID, t.GetState())

	for {
		select {
		case event := <-t.transitionChan:
			t.handleEvent(event)
		case <-t.doneChan:
			fmt.Printf("Task %s: State machine stopped\n", t.ID)
			return
		}
	}
}

// handleEvent processes an event and updates the state accordingly
func (t *Task) handleEvent(event Event) {
	currentState := t.GetState()
	newState := currentState

	processTime := time.Duration(rand.Intn(1000)) * time.Millisecond
	time.Sleep(processTime)

	fmt.Printf("Task %s: Processing event %s from state %s\n", t.ID, event, currentState)

	switch currentState {
	case Pending:
		if event == StartProvision {
			newState = Provisioning
			go func() {
				time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)
				if rand.Float32() < 0.8 { // 80% success rate
					t.SendEvent(ProvisionSuccess)
				} else {
					t.SendEvent(ProvisionFail)
				}
			}()
		}
	case Provisioning:
		if event == ProvisionSuccess {
			newState = Stopped
		} else if event == ProvisionFail || event == Fail {
			newState = Failed
		}
	case Stopped:
		if event == StartVM {
			newState = Running
		} else if event == TerminateVM {
			newState = Terminated
		}
	case Running:
		if event == PauseVM {
			newState = Paused
		} else if event == StopVM {
			newState = Stopping
		} else if event == Fail {
			newState = Failed
		}
	case Paused:
		if event == ResumeVM {
			newState = Running
		} else if event == StopVM {
			newState = Stopping
		} else if event == TerminateVM {
			newState = Terminated
		}
	case Stopping:
		if event == ProvisionSuccess {
			newState = Stopped
		} else if event == Fail {
			newState = Failed
		}
	case Failed:
		if event == TerminateVM {
			newState = Terminated
		}
	case Terminated:

	}

	if newState != currentState {
		t.setState(newState)
		fmt.Printf("Task %s: Transitioned from %s to %s\n", t.ID, currentState, newState)
	} else {
		fmt.Printf("Task %s: Event %s did not trigger state change from %s\n", t.ID, event, currentState)
	}
}

// TaskManager manages multiple VM tasks
type TaskManager struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

// NewTaskManager creates a new task manager
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*Task),
	}
}

// CreateTask creates a new task and starts its state machine
func (tm *TaskManager) CreateTask(id string) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task := NewTask(id)
	task.Start()
	tm.tasks[id] = task
	return task
}

// GetTask retrieves a task by ID
func (tm *TaskManager) GetTask(id string) (*Task, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[id]
	return task, exists
}

// DeleteTask stops and removes a task
func (tm *TaskManager) DeleteTask(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[id]; exists {
		task.Stop()
		delete(tm.tasks, id)
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	manager := NewTaskManager()

	numTasks := 5
	for i := 1; i <= numTasks; i++ {
		taskID := fmt.Sprintf("vm-%d", i)
		task := manager.CreateTask(taskID)

		switch i % 3 {
		case 0:
			task.SendEvent(StartProvision)
		case 1:
			task.SendEvent(StartProvision)
			time.Sleep(500 * time.Millisecond)
		case 2:
			task.SendEvent(StartProvision)
			time.Sleep(500 * time.Millisecond)
		}
	}

	time.Sleep(3 * time.Second)

	if task, exists := manager.GetTask("vm-1"); exists {
		if task.GetState() == Stopped {
			task.SendEvent(StartVM)
			time.Sleep(500 * time.Millisecond)
			task.SendEvent(PauseVM)
			time.Sleep(500 * time.Millisecond)
			task.SendEvent(ResumeVM)
			time.Sleep(500 * time.Millisecond)
			task.SendEvent(StopVM)
		} else if task.GetState() == Running {
			task.SendEvent(StopVM)
		}
	}

	time.Sleep(2 * time.Second)

	fmt.Println("\nFinal VM States:")
	for i := 1; i <= numTasks; i++ {
		taskID := fmt.Sprintf("vm-%d", i)
		if task, exists := manager.GetTask(taskID); exists {
			fmt.Printf("Task %s: %s\n", taskID, task.GetState())
		}
	}

	for i := 1; i <= numTasks; i++ {
		taskID := fmt.Sprintf("vm-%d", i)
		manager.DeleteTask(taskID)
	}
}
