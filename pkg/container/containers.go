package container

import "time"

type Status string

const (
	StatusCreated    Status = "created"
	StatusRunning    Status = "running"
	StatusPaused     Status = "paused"
	StatusExited     Status = "exited"
	StatusRestarting Status = "restarting"
	StatusRemoving   Status = "removing"
	StatusDead       Status = "dead"
)

type Container struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	Command    []string          `json:"command"` // entrypoint + args
	Env        []string          `json:"env"`     // "value=string"
	Labels     map[string]string `json:"labels"`
	BundlePath string            `json:"bundle_path"` // dir that contains rootfs + config
	RootFS     string            `json:"rootfs"`      // path to container's rootfs
	CreatedAt  time.Time         `json:"created_at"`
}

type State struct {
	Status   Status    `json:"status"`
	PID      int       `json:"pid"`
	ExitCode int       `json:"exit_code,omitempty"`
	ExitedAt time.Time `json:"exited_at,omitempty"`
}

type ContainerListItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Status    Status    `json:"status"`
	RootFS    string    `json:"rootfs"`
	CreatedAt time.Time `json:"created_at"`
}
