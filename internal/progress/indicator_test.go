package progress

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewIndicator(t *testing.T) {
	t.Run("quiet mode returns NoOpIndicator", func(t *testing.T) {
		indicator := NewIndicator(true)
		_, ok := indicator.(*NoOpIndicator)
		assert.True(t, ok)
	})

	t.Run("non-quiet mode returns SpinnerIndicator", func(t *testing.T) {
		indicator := NewIndicator(false)
		_, ok := indicator.(*SpinnerIndicator)
		assert.True(t, ok)
	})
}

func TestNoOpIndicator(t *testing.T) {
	indicator := &NoOpIndicator{}
	
	// All methods should do nothing without panicking
	indicator.Start("test")
	indicator.Update(1, 10, "progress")
	indicator.Complete("done")
	indicator.Error(errors.New("test error"))
	indicator.Stop()
	
	// If we got here without panic, test passes
	assert.True(t, true)
}

func TestSpinnerIndicator_Start(t *testing.T) {
	buf := &bytes.Buffer{}
	indicator := NewSpinnerIndicator(buf)
	
	indicator.Start("Loading...")
	time.Sleep(150 * time.Millisecond) // Let spinner spin at least once
	indicator.Stop()
	
	output := buf.String()
	assert.Contains(t, output, "Loading...")
	// Should contain at least one spinner character
	hasSpinner := false
	for _, char := range []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"} {
		if strings.Contains(output, char) {
			hasSpinner = true
			break
		}
	}
	assert.True(t, hasSpinner, "Output should contain spinner character")
}

func TestSpinnerIndicator_Update(t *testing.T) {
	buf := &bytes.Buffer{}
	indicator := NewSpinnerIndicator(buf)
	
	indicator.Start("Processing...")
	time.Sleep(50 * time.Millisecond)
	
	indicator.Update(5, 10, "Processing files")
	time.Sleep(150 * time.Millisecond)
	indicator.Stop()
	
	output := buf.String()
	assert.Contains(t, output, "Processing files")
	assert.Contains(t, output, "50%") // 5/10 = 50%
}

func TestSpinnerIndicator_UpdateWithZeroTotal(t *testing.T) {
	buf := &bytes.Buffer{}
	indicator := NewSpinnerIndicator(buf)
	
	indicator.Start("Processing...")
	indicator.Update(5, 0, "Processing files")
	time.Sleep(150 * time.Millisecond)
	indicator.Stop()
	
	output := buf.String()
	assert.Contains(t, output, "Processing files")
	assert.NotContains(t, output, "%") // Should not show percentage
}

func TestSpinnerIndicator_Complete(t *testing.T) {
	buf := &bytes.Buffer{}
	indicator := NewSpinnerIndicator(buf)
	
	indicator.Start("Loading...")
	time.Sleep(50 * time.Millisecond)
	indicator.Complete("Task completed successfully")
	
	output := buf.String()
	assert.Contains(t, output, "✓ Task completed successfully")
}

func TestSpinnerIndicator_Error(t *testing.T) {
	buf := &bytes.Buffer{}
	indicator := NewSpinnerIndicator(buf)
	
	indicator.Start("Loading...")
	time.Sleep(50 * time.Millisecond)
	indicator.Error(errors.New("something went wrong"))
	
	output := buf.String()
	assert.Contains(t, output, "✗ something went wrong")
}

func TestSpinnerIndicator_MultipleStart(t *testing.T) {
	buf := &bytes.Buffer{}
	indicator := NewSpinnerIndicator(buf)
	
	// Starting multiple times should not create multiple goroutines
	indicator.Start("First")
	indicator.Start("Second") // Should be ignored
	time.Sleep(150 * time.Millisecond)
	indicator.Stop()
	
	output := buf.String()
	assert.Contains(t, output, "First")
	assert.NotContains(t, output, "Second")
}

func TestSpinnerIndicator_ConcurrentAccess(t *testing.T) {
	buf := &bytes.Buffer{}
	indicator := NewSpinnerIndicator(buf)
	
	var wg sync.WaitGroup
	wg.Add(3)
	
	// Start spinner
	go func() {
		defer wg.Done()
		indicator.Start("Starting...")
	}()
	
	// Update from another goroutine
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		for i := 0; i < 5; i++ {
			indicator.Update(i, 5, "Progress")
			time.Sleep(20 * time.Millisecond)
		}
	}()
	
	// Complete from another goroutine
	go func() {
		defer wg.Done()
		time.Sleep(200 * time.Millisecond)
		indicator.Complete("Done")
	}()
	
	wg.Wait()
	indicator.Stop()
	
	// If we got here without deadlock or panic, concurrency is handled correctly
	output := buf.String()
	assert.Contains(t, output, "✓ Done")
}

func TestSpinnerIndicator_StopWithoutStart(t *testing.T) {
	buf := &bytes.Buffer{}
	indicator := NewSpinnerIndicator(buf)
	
	// Stopping without starting should not panic
	indicator.Stop()
	
	// Output should be empty
	assert.Empty(t, buf.String())
}

func TestSpinnerIndicator_NilWriter(t *testing.T) {
	// Should use os.Stderr as default
	indicator := NewSpinnerIndicator(nil)
	assert.NotNil(t, indicator.writer)
	
	// Should not panic
	indicator.Start("Test")
	indicator.Stop()
}

func TestSpinnerIndicator_ClearLine(t *testing.T) {
	buf := &bytes.Buffer{}
	indicator := NewSpinnerIndicator(buf)
	
	// Write something first
	buf.WriteString("Some existing text")
	
	indicator.Start("New message")
	time.Sleep(50 * time.Millisecond)
	indicator.Complete("Completed")
	
	output := buf.String()
	// Should contain carriage returns and spaces for clearing
	assert.Contains(t, output, "\r")
	assert.Contains(t, output, " ")
	assert.Contains(t, output, "✓ Completed")
}