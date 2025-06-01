package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Indicator は進捗表示のインターフェース
type Indicator interface {
	Start(message string)
	Update(current, total int, message string)
	Complete(message string)
	Error(err error)
	Stop()
}

// SpinnerIndicator はスピナーを表示する進捗インジケーター
type SpinnerIndicator struct {
	writer    io.Writer
	spinner   []string
	current   int
	message   string
	isRunning bool
	mu        sync.Mutex
	stopCh    chan struct{}
}

// NewSpinnerIndicator は新しいスピナーインジケーターを作成します
func NewSpinnerIndicator(writer io.Writer) *SpinnerIndicator {
	if writer == nil {
		writer = os.Stderr
	}
	
	return &SpinnerIndicator{
		writer:  writer,
		spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stopCh:  make(chan struct{}),
	}
}

// Start は進捗表示を開始します
func (s *SpinnerIndicator) Start(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.isRunning {
		return
	}
	
	s.message = message
	s.isRunning = true
	s.current = 0
	
	go s.spin()
}

// spin はスピナーアニメーションを実行します
func (s *SpinnerIndicator) spin() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if s.isRunning {
				frame := s.spinner[s.current%len(s.spinner)]
				fmt.Fprintf(s.writer, "\r%s %s", frame, s.message)
				s.current++
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

// Update は進捗を更新します
func (s *SpinnerIndicator) Update(current, total int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if total > 0 {
		percentage := float64(current) / float64(total) * 100
		s.message = fmt.Sprintf("%s (%.0f%%)", message, percentage)
	} else {
		s.message = message
	}
}

// Complete は進捗を完了状態にします
func (s *SpinnerIndicator) Complete(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.clearLine()
	fmt.Fprintf(s.writer, "✓ %s\n", message)
	s.isRunning = false
}

// Error はエラー状態を表示します
func (s *SpinnerIndicator) Error(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.clearLine()
	fmt.Fprintf(s.writer, "✗ %s\n", err.Error())
	s.isRunning = false
}

// Stop は進捗表示を停止します
func (s *SpinnerIndicator) Stop() {
	s.mu.Lock()
	if s.isRunning {
		s.isRunning = false
		s.clearLine()
	}
	s.mu.Unlock()
	
	close(s.stopCh)
}

// clearLine は現在の行をクリアします
func (s *SpinnerIndicator) clearLine() {
	fmt.Fprintf(s.writer, "\r%s\r", strings.Repeat(" ", 80))
}

// NoOpIndicator は何も表示しない進捗インジケーター（quiet mode用）
type NoOpIndicator struct{}

func (n *NoOpIndicator) Start(message string)                   {}
func (n *NoOpIndicator) Update(current, total int, message string) {}
func (n *NoOpIndicator) Complete(message string)                {}
func (n *NoOpIndicator) Error(err error)                        {}
func (n *NoOpIndicator) Stop()                                   {}

// NewIndicator は設定に基づいて適切なインジケーターを作成します
func NewIndicator(quiet bool) Indicator {
	if quiet {
		return &NoOpIndicator{}
	}
	return NewSpinnerIndicator(os.Stderr)
}