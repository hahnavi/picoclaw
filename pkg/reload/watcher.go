// Package reload provides hot reload functionality for PicoClaw.
// It watches configuration, skills, and bootstrap files for changes
// and triggers appropriate reload actions.
package reload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// WatchEventType represents the type of file change event.
type WatchEventType int

const (
	WatchEventConfig WatchEventType = iota
	WatchEventSkill
	WatchEventBootstrap
)

func (et WatchEventType) String() string {
	switch et {
	case WatchEventConfig:
		return "config"
	case WatchEventSkill:
		return "skill"
	case WatchEventBootstrap:
		return "bootstrap"
	default:
		return "unknown"
	}
}

// WatchEvent represents a file change event.
type WatchEvent struct {
	Type      WatchEventType
	Path      string
	Timestamp time.Time
}

// WatcherConfig configures the file watcher.
type WatcherConfig struct {
	ConfigPath     string
	WorkspacePath  string
	WatchSkills    bool
	WatchBootstrap bool
}

// FileWatcher watches files for changes and emits debounced events.
type FileWatcher struct {
	watcher  *fsnotify.Watcher
	events   chan WatchEvent
	debounce time.Duration
	timers   map[string]*time.Timer
	mu       sync.Mutex
	config   WatcherConfig
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewFileWatcher creates a new file watcher.
func NewFileWatcher(config WatcherConfig, debounce time.Duration) (*FileWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &FileWatcher{
		watcher:  fsWatcher,
		events:   make(chan WatchEvent, 10),
		debounce: debounce,
		timers:   make(map[string]*time.Timer),
		config:   config,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Start begins watching files for changes.
func (fw *FileWatcher) Start(ctx context.Context) error {
	logger.InfoCF("reload", "Starting file watcher",
		map[string]interface{}{
			"config_path":     fw.config.ConfigPath,
			"workspace_path":  fw.config.WorkspacePath,
			"watch_skills":    fw.config.WatchSkills,
			"watch_bootstrap": fw.config.WatchBootstrap,
			"debounce_ms":     fw.debounce.Milliseconds(),
		})

	// Watch config file
	if fw.config.ConfigPath != "" {
		if err := fw.watchFile(fw.config.ConfigPath, WatchEventConfig); err != nil {
			logger.WarnC("reload", fmt.Sprintf("Failed to watch config file: %v", err))
		}
	}

	// Watch bootstrap files
	if fw.config.WatchBootstrap {
		bootstrapFiles := []string{"IDENTITY.md", "SOUL.md", "AGENTS.md", "USER.md"}
		for _, file := range bootstrapFiles {
			path := filepath.Join(fw.config.WorkspacePath, file)
			if err := fw.watchFile(path, WatchEventBootstrap); err != nil {
				logger.WarnC("reload", fmt.Sprintf("Failed to watch bootstrap file %s: %v", file, err))
			}
		}
	}

	// Watch skills directory
	if fw.config.WatchSkills {
		if err := fw.watchSkillsGlob(); err != nil {
			logger.WarnC("reload", fmt.Sprintf("Failed to watch skills: %v", err))
		}
	}

	fw.wg.Add(1)
	go fw.eventLoop()

	return nil
}

// eventLoop processes fsnotify events and emits debounced WatchEvents.
func (fw *FileWatcher) eventLoop() {
	defer fw.wg.Done()

	for {
		select {
		case <-fw.ctx.Done():
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleFsEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			logger.ErrorC("reload", fmt.Sprintf("Watcher error: %v", err))
		}
	}
}

// handleFsEvent handles a single fsnotify event with debouncing.
func (fw *FileWatcher) handleFsEvent(event fsnotify.Event) {
	// Determine event type
	eventType := fw.determineEventType(event.Name)
	if eventType == -1 {
		return // Not a file we care about
	}

	// Only process write, create, and remove events
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	logger.DebugC("reload", fmt.Sprintf("File event: %s on %s", event.Op, event.Name))

	// Debounce: cancel existing timer for this path and start a new one
	fw.mu.Lock()
	path := event.Name

	if timer, exists := fw.timers[path]; exists {
		timer.Stop()
		delete(fw.timers, path)
	}

	fw.timers[path] = time.AfterFunc(fw.debounce, func() {
		fw.mu.Lock()
		delete(fw.timers, path)
		fw.mu.Unlock()

		// For remove/rename events, re-watch the file if it gets recreated
		if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			// Try to re-watch after a short delay
			time.Sleep(100 * time.Millisecond)
			if _, err := os.Stat(path); err == nil {
				fw.watcher.Add(path)
				logger.DebugC("reload", fmt.Sprintf("Re-watched file after recreate: %s", path))
			}
		}

		// Emit the event
		select {
		case fw.events <- WatchEvent{
			Type:      eventType,
			Path:      path,
			Timestamp: time.Now(),
		}:
			logger.InfoC("reload", fmt.Sprintf("Emitted %s event for %s", eventType, path))
		case <-fw.ctx.Done():
			return
		}
	})
	fw.mu.Unlock()
}

// determineEventType determines the WatchEventType for a given path.
func (fw *FileWatcher) determineEventType(path string) WatchEventType {
	// Check if it's the config file
	if filepath.Clean(path) == filepath.Clean(fw.config.ConfigPath) {
		return WatchEventConfig
	}

	// Check if it's a bootstrap file
	if fw.config.WatchBootstrap {
		base := filepath.Base(path)
		if base == "IDENTITY.md" || base == "SOUL.md" || base == "AGENTS.md" || base == "USER.md" {
			dir := filepath.Dir(path)
			if filepath.Clean(dir) == filepath.Clean(fw.config.WorkspacePath) {
				return WatchEventBootstrap
			}
		}
	}

	// Check if it's a skill file
	if fw.config.WatchSkills {
		if filepath.Base(path) == "SKILL.md" {
			// Check if it's in a skills directory
			parentDir := filepath.Dir(path)
			grandparentDir := filepath.Dir(parentDir)
			if filepath.Base(grandparentDir) == "skills" {
				return WatchEventSkill
			}
		}
	}

	return -1
}

// watchFile adds a single file to the watcher.
func (fw *FileWatcher) watchFile(path string, eventType WatchEventType) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			logger.WarnC("reload", fmt.Sprintf("File does not exist, skipping: %s", path))
			return nil
		}
		return err
	}

	if err := fw.watcher.Add(path); err != nil {
		return err
	}

	logger.DebugC("reload", fmt.Sprintf("Watching %s file: %s", eventType, path))
	return nil
}

// watchSkillsGlob watches the skills directory using glob patterns.
func (fw *FileWatcher) watchSkillsGlob() error {
	// Watch workspace skills
	workspaceSkills := filepath.Join(fw.config.WorkspacePath, "skills")
	if err := fw.watchSkillsDirectory(workspaceSkills); err != nil {
		logger.WarnC("reload", fmt.Sprintf("Failed to watch workspace skills: %v", err))
	}

	// Note: We don't watch global or builtin skills as they're not expected to change at runtime
	return nil
}

// watchSkillsDirectory watches a skills directory and all its subdirectories.
func (fw *FileWatcher) watchSkillsDirectory(skillsDir string) error {
	if _, err := os.Stat(skillsDir); err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist yet, that's okay
		}
		return err
	}

	// Watch the directory itself for new skill directories
	if err := fw.watcher.Add(skillsDir); err != nil {
		logger.WarnC("reload", fmt.Sprintf("Failed to watch skills directory %s: %v", skillsDir, err))
	}

	// Watch existing skill subdirectories
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(skillsDir, entry.Name())
			skillFile := filepath.Join(skillPath, "SKILL.md")

			// Watch the skill directory for changes
			if err := fw.watcher.Add(skillPath); err == nil {
				logger.DebugC("reload", fmt.Sprintf("Watching skill directory: %s", skillPath))
			}

			// Watch the SKILL.md file specifically
			if _, err := os.Stat(skillFile); err == nil {
				if err := fw.watcher.Add(skillFile); err == nil {
					logger.DebugC("reload", fmt.Sprintf("Watching skill file: %s", skillFile))
				}
			}
		}
	}

	return nil
}

// Events returns the channel of watch events.
func (fw *FileWatcher) Events() <-chan WatchEvent {
	return fw.events
}

// Close stops the watcher and cleans up resources.
func (fw *FileWatcher) Close() error {
	logger.InfoC("reload", "Closing file watcher")

	fw.cancel()

	// Stop all timers
	fw.mu.Lock()
	for path, timer := range fw.timers {
		timer.Stop()
		delete(fw.timers, path)
		logger.DebugC("reload", fmt.Sprintf("Stopped timer for: %s", path))
	}
	fw.mu.Unlock()

	// Wait for event loop to finish
	fw.wg.Wait()

	// Close events channel
	close(fw.events)

	// Close fsnotify watcher
	if fw.watcher != nil {
		if err := fw.watcher.Close(); err != nil {
			logger.ErrorC("reload", fmt.Sprintf("Error closing watcher: %v", err))
			return err
		}
	}

	return nil
}
