package common

import (
	"fmt"
	"testing"
)

func TestSliceReordering(t *testing.T) {
	slice1 := []int{0, 1, 2, 3, 4, 5}
	ReorderSlice(slice1, 0, 3)
	fmt.Printf("result %v\n", slice1)
	if slice1[0] != 1 || slice1[1] != 2 || slice1[2] != 3 || slice1[3] != 0 || slice1[4] != 4 || slice1[5] != 5 {
		fmt.Printf("Failed to reorder %v", slice1)
	}
	slice1 = []int{0, 1, 2, 3, 4, 5}
	ReorderSlice(slice1, 3, 0)
	fmt.Printf("result %v\n", slice1)
	if slice1[0] != 3 || slice1[1] != 0 || slice1[2] != 1 || slice1[3] != 2 || slice1[4] != 4 || slice1[5] != 5 {
		fmt.Printf("Failed to reorder %v", slice1)
	}
	slice1 = []int{0, 1, 2, 3, 4, 5}
	ReorderSlice(slice1, 0, 1)
	fmt.Printf("result %v\n", slice1)
	if slice1[0] != 1 || slice1[1] != 0 || slice1[2] != 2 || slice1[3] != 2 || slice1[4] != 4 || slice1[5] != 5 {
		fmt.Printf("Failed to reorder %v", slice1)
	}
	slice1 = []int{0, 1, 2, 3, 4, 5}
	ReorderSlice(slice1, 1, 0)
	fmt.Printf("result %v\n", slice1)
	if slice1[0] != 1 || slice1[1] != 0 || slice1[2] != 2 || slice1[3] != 2 || slice1[4] != 4 || slice1[5] != 5 {
		fmt.Printf("Failed to reorder %v", slice1)
	}
}

func TestExtractNameFromPath(t *testing.T) {
	tests := []struct {
		inputPath      string
		expectedName   string
		expectedParent string
	}{
		{"/home/user/docs/file.txt", "file.txt", "/home/user/docs"},
		{"/home/user/docs/", "", "/home/user/docs/"},
		{"file.txt", "file.txt", ""},
		{"/", "", "/"},
		{"", "", ""},
	}

	for _, test := range tests {
		name, parent := ExtractNameFromPath(test.inputPath)
		if name != test.expectedName || parent != test.expectedParent {
			t.Errorf("ExtractNameFromPath(%q) = (%q, %q); want (%q, %q)",
				test.inputPath, name, parent, test.expectedName, test.expectedParent)
		}
	}
}
func TestLongTasksContext(t *testing.T) {
	t.Run("AddTask", func(t *testing.T) {
		ltc := &LongTasksContext{queue: make([]*LongTask, 0)}
		task := ltc.AddTask("Task1")

		if len(ltc.queue) != 1 {
			t.Errorf("expected queue length to be 1, got %d", len(ltc.queue))
		}
		if ltc.queue[0] != task {
			t.Errorf("expected task to be added to queue")
		}
		if task.Name != "Task1" {
			t.Errorf("expected task name to be 'Task1', got %q", task.Name)
		}
	})

	t.Run("RemoveTask", func(t *testing.T) {
		ltc := &LongTasksContext{queue: make([]*LongTask, 0)}
		ltc.AddTask("Task1")
		ltc.AddTask("Task2")
		ltc.RemoveTask("Task1")

		if len(ltc.queue) != 1 {
			t.Errorf("expected queue length to be 1, got %d", len(ltc.queue))
		}
		if ltc.queue[0].Name != "Task2" {
			t.Errorf("expected remaining task to be 'Task2', got %q", ltc.queue[0].Name)
		}
	})

	t.Run("GetFirstTask", func(t *testing.T) {
		ltc := &LongTasksContext{queue: make([]*LongTask, 0)}
		if task := ltc.GetFirstTask(); task != nil {
			t.Errorf("expected nil, got %v", task)
		}

		ltc.AddTask("Task1")
		ltc.AddTask("Task2")
		task := ltc.GetFirstTask()

		if task == nil || task.Name != "Task1" {
			t.Errorf("expected first task to be 'Task1', got %v", task)
		}
	})

	t.Run("Task Done", func(t *testing.T) {
		ltc := &LongTasksContext{queue: make([]*LongTask, 0)}
		task := ltc.AddTask("Task1")
		task.Done()

		if len(ltc.queue) != 0 {
			t.Errorf("expected queue length to be 0, got %d", len(ltc.queue))
		}
	})

	t.Run("Task Update", func(t *testing.T) {
		ltc := &LongTasksContext{queue: make([]*LongTask, 0)}
		task := ltc.AddTask("Task1")
		task.Step = 0.5
		task.Update("half")

		if task.Progress != 0.5 {
			t.Errorf("expected progress to be 0.5, got %f", task.Progress)
		}

		task.Update("done")
		if len(ltc.queue) != 0 {
			t.Errorf("expected queue length to be 0 after task completion, got %d", len(ltc.queue))
		}
	})
}
