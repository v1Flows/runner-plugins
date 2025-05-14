package main

import (
	"context"
	"errors"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apenella/go-ansible/v2/pkg/execute"
	"github.com/apenella/go-ansible/v2/pkg/execute/configuration"
	"github.com/apenella/go-ansible/v2/pkg/playbook"

	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"
	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

// Map ANSI color codes to models.Line.Color values
var ansiToLineColor = map[string]string{
	"\033[0;31m": "danger",     // Red
	"\033[0;32m": "success",    // Green
	"\033[0;33m": "warning",    // Yellow
	"\033[0;34m": "primary",    // Blue
	"\033[0;35m": "purple-500", // Purple
	"\033[0;36m": "cyan-500",   // Cyan
	"\033[0;0m":  "",           // Reset
}

var (
	taskCancels   = make(map[string]context.CancelFunc)
	taskCancelsMu sync.Mutex
)

// Function to strip ANSI color codes and map them to models.Line.Color
func parseAnsiColor(output string) (string, string) {
	for ansiCode, lineColor := range ansiToLineColor {
		if strings.Contains(output, ansiCode) {
			// Remove the ANSI code from the output
			cleanOutput := strings.ReplaceAll(output, ansiCode, "")
			cleanOutput = strings.ReplaceAll(cleanOutput, "\033[0m", "") // Remove reset code
			return cleanOutput, lineColor
		} else if strings.Contains(output, "Whoops! context canceled") {
			return output, "danger" // Handle specific error message
		}
	}
	return output, "" // Default: no color
}

type CustomWriter struct {
	OutputFunc func(string, string)
}

func (cw *CustomWriter) Write(p []byte) (n int, err error) {
	output := string(p)
	cleanOutput, color := parseAnsiColor(output)
	cw.OutputFunc(cleanOutput, color)
	return len(p), nil
}

func handleOutput(output string, color string, request plugins.ExecuteTaskRequest) error {
	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Ansible Playbook",
				Lines: []models.Line{
					{
						Content:   output,
						Color:     color,
						Timestamp: time.Now(),
					},
				},
			},
		},
		Status: "running",
	}, request.Platform)
	if err != nil {
		return err
	}

	return nil
}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	ctx, cancel := context.WithCancel(context.Background())
	stepID := request.Step.ID.String()

	// Store cancel func
	taskCancelsMu.Lock()
	taskCancels[stepID] = cancel
	taskCancelsMu.Unlock()
	defer func() {
		taskCancelsMu.Lock()
		delete(taskCancels, stepID)
		taskCancelsMu.Unlock()
	}()

	play := ""
	inventory := ""
	become := false
	limit := ""
	check := false
	diff := false
	user := ""
	password := ""
	becomeUser := ""
	becomePass := ""
	verbose := 0
	private_key := ""

	// access action params
	for _, param := range request.Step.Action.Params {
		if param.Key == "playbook" {
			if strings.Contains(param.Value, "/") {
				play = request.Workspace + param.Value
			} else {
				play = param.Value
			}
		}
		if param.Key == "inventory" {
			// if inventory is a path prefix with workspace
			if strings.Contains(param.Value, "/") {
				inventory = request.Workspace + param.Value
			} else {
				inventory = param.Value
			}
		}
		if param.Key == "become" {
			become = param.Value == "true"
		}
		if param.Key == "limit" {
			limit = param.Value
		}
		if param.Key == "check" {
			check = param.Value == "true"
		}
		if param.Key == "diff" {
			diff = param.Value == "true"
		}
		if param.Key == "user" {
			user = param.Value
		}
		if param.Key == "password" {
			password = param.Value
		}
		if param.Key == "become_user" {
			becomeUser = param.Value
		}
		if param.Key == "become_pass" {
			becomePass = param.Value
		}
		if param.Key == "verbose" {
			verbose, _ = strconv.Atoi(param.Value)
		}
		if param.Key == "private_key" {
			private_key = param.Value
		}
	}

	// Check for cancellation before each major step
	if ctx.Err() != nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Cancel",
					Lines: []models.Line{
						{
							Content:   "Action canceled",
							Color:     "danger",
							Timestamp: time.Now(),
						},
					},
				},
			},
			Status:     "canceled",
			FinishedAt: time.Now(),
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}

		return plugins.Response{Success: false, Canceled: true}, nil
	}

	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Ansible Playbook",
				Lines: []models.Line{
					{
						Content:   "Starting Ansible Playbook",
						Timestamp: time.Now(),
					},
					{
						Content:   "Playbook: " + play,
						Timestamp: time.Now(),
					},
					{
						Content:   "Inventory: " + inventory,
						Timestamp: time.Now(),
					},
				},
			},
		},
		Status:    "running",
		StartedAt: time.Now(),
	}, request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	// check if playbook file exists
	if _, err := os.Stat(play); errors.Is(err, os.ErrNotExist) {
		err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Ansible Playbook",
					Lines: []models.Line{
						{
							Content:   "Playbook file does not exist",
							Color:     "danger",
							Timestamp: time.Now(),
						},
						{
							Content:   err.Error(),
							Color:     "danger",
							Timestamp: time.Now(),
						},
					},
				},
			},
			Status:     "error",
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
		return plugins.Response{
			Success: false,
		}, errors.New("playbook file does not exist")
	}

	// if inventory is a file check and not a comma separated list check if file exists
	if !strings.Contains(inventory, ",") && net.ParseIP(inventory) == nil {
		if _, err := os.Stat(inventory); errors.Is(err, os.ErrNotExist) {
			err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Ansible Playbook",
						Lines: []models.Line{
							{
								Content:   "Inventory file does not exist",
								Color:     "danger",
								Timestamp: time.Now(),
							},
							{
								Content:   err.Error(),
								Color:     "danger",
								Timestamp: time.Now(),
							},
						},
					},
				},
				Status:     "error",
				StartedAt:  time.Now(),
				FinishedAt: time.Now(),
			}, request.Platform)
			if err != nil {
				return plugins.Response{
					Success: false,
				}, err
			}
			return plugins.Response{
				Success: false,
			}, errors.New("inventory file does not exist")
		}
	}

	ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
		Connection:    "ssh",
		Inventory:     inventory,
		Become:        become,
		Limit:         limit,
		Check:         check,
		Diff:          diff,
		User:          user,
		BecomeUser:    becomeUser,
		SSHCommonArgs: "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null",
		ExtraVars: map[string]interface{}{
			"ansible_password":    password,
			"ansible_become_pass": becomePass,
		},
		PrivateKey: private_key,
	}

	if verbose == 1 {
		ansiblePlaybookOptions.Verbose = true
		ansiblePlaybookOptions.VerboseV = true
	} else if verbose == 2 {
		ansiblePlaybookOptions.Verbose = true
		ansiblePlaybookOptions.VerboseVV = true
	} else if verbose == 3 {
		ansiblePlaybookOptions.Verbose = true
		ansiblePlaybookOptions.VerboseVVV = true
	} else if verbose == 4 {
		ansiblePlaybookOptions.Verbose = true
		ansiblePlaybookOptions.VerboseVVVV = true
	}

	playbookCmd := playbook.NewAnsiblePlaybookCmd(
		playbook.WithPlaybooks(play),
		playbook.WithPlaybookOptions(ansiblePlaybookOptions),
	)

	// Use a custom writer to capture output
	customWriter := &CustomWriter{
		OutputFunc: func(output string, color string) {
			err := handleOutput(output, color, request)
			if err != nil {
				_ = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
					ID: request.Step.ID,
					Messages: []models.Message{
						{
							Title: "Ansible Playbook",
							Lines: []models.Line{
								{
									Content:   "Ansible Playbook failed",
									Color:     "danger",
									Timestamp: time.Now(),
								},
								{
									Content:   err.Error(),
									Color:     "danger",
									Timestamp: time.Now(),
								},
							},
						},
					},
					Status:     "error",
					FinishedAt: time.Now(),
				}, request.Platform)
			}
		},
	}

	exec := configuration.NewAnsibleWithConfigurationSettingsExecute(
		execute.NewDefaultExecute(
			execute.WithCmd(playbookCmd),
			execute.WithErrorEnrich(playbook.NewAnsiblePlaybookErrorEnrich()),
			execute.WithWrite(customWriter), // Redirect both stdout and stderr to custom writer.
		),
		configuration.WithAnsibleForceColor(),
	)

	// Check for cancellation before each major step
	if ctx.Err() != nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Cancel",
					Lines: []models.Line{
						{
							Content:   "Action canceled",
							Color:     "danger",
							Timestamp: time.Now(),
						},
					},
				},
			},
			Status:     "canceled",
			FinishedAt: time.Now(),
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}

		return plugins.Response{Success: false, Canceled: true}, nil
	}

	err = exec.Execute(ctx)
	if err != nil {
		err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Ansible Playbook",
					Lines: []models.Line{
						{
							Content:   "Ansible Playbook failed",
							Color:     "danger",
							Timestamp: time.Now(),
						},
						{
							Content:   err.Error(),
							Color:     "danger",
							Timestamp: time.Now(),
						},
					},
				},
			},
			Status:     "error",
			FinishedAt: time.Now(),
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}

		return plugins.Response{
			Success: false,
		}, err
	}

	// Check for cancellation before each major step
	if ctx.Err() != nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Cancel",
					Lines: []models.Line{
						{
							Content:   "Action canceled",
							Color:     "danger",
							Timestamp: time.Now(),
						},
					},
				},
			},
			Status:     "canceled",
			FinishedAt: time.Now(),
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}

		return plugins.Response{Success: false, Canceled: true}, nil
	}

	// update the step with the messages
	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Ansible Playbook",
				Lines: []models.Line{
					{
						Content:   "Ansible Playbook executed successfully",
						Color:     "success",
						Timestamp: time.Now(),
					},
				},
			},
		},
		Status:     "success",
		FinishedAt: time.Now(),
	}, request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	return plugins.Response{
		Success: true,
	}, nil
}

func (p *Plugin) CancelTask(request plugins.CancelTaskRequest) (plugins.Response, error) {
	stepID := request.Step.ID.String()
	taskCancelsMu.Lock()
	cancel, ok := taskCancels[stepID]
	taskCancelsMu.Unlock()

	if !ok {
		return plugins.Response{
			Success: false,
		}, errors.New("task not found")
	}

	cancel()
	return plugins.Response{Success: true}, nil
}

func (p *Plugin) EndpointRequest(request plugins.EndpointRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, errors.New("not implemented")
}

func (p *Plugin) Info(request plugins.InfoRequest) (models.Plugin, error) {
	var plugin = models.Plugin{
		Name:    "Ansible",
		Type:    "action",
		Version: "1.3.0",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Ansible",
			Description: "Execute Ansible Playbook",
			Plugin:      "ansible",
			Icon:        "mdi:ansible",
			Category:    "Automation",
			Params: []models.Params{
				{
					Key:         "playbook",
					Title:       "Playbook",
					Category:    "General",
					Type:        "text",
					Default:     "",
					Required:    true,
					Description: "Path to the playbook file. The path prefix is the workspace directory: " + request.Workspace,
				},
				{
					Key:         "inventory",
					Title:       "Inventory",
					Category:    "General",
					Type:        "text",
					Default:     "",
					Required:    true,
					Description: "Path to the inventory file or comma separated host list. The path prefix is the workspace directory: " + request.Workspace,
				},
				{
					Key:         "user",
					Title:       "User",
					Category:    "Authentication",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Connect as this user",
				},
				{
					Key:         "password",
					Title:       "Password",
					Category:    "Authentication",
					Type:        "password",
					Default:     "",
					Required:    false,
					Description: "Connection user password",
				},
				{
					Key:         "private_key",
					Title:       "Private Key",
					Category:    "Authentication",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Path to the private key file",
				},
				{
					Key:         "limit",
					Title:       "Limit",
					Category:    "General",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Further limit selected hosts to an additional pattern",
				},
				{
					Key:         "become",
					Title:       "Become",
					Category:    "Sudo",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Run playbook with become",
				},
				{
					Key:         "become_user",
					Title:       "Become User",
					Category:    "Sudo",
					Type:        "text",
					Default:     "root",
					Required:    false,
					Description: "User to run become tasks with",
				},
				{
					Key:         "become_pass",
					Title:       "Become Password",
					Category:    "Sudo",
					Type:        "password",
					Default:     "",
					Required:    false,
					Description: "Become user password",
				},
				{
					Key:         "check",
					Title:       "Check",
					Category:    "Utility",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Don't make any changes; instead, try to predict some of the changes that may occur",
				},
				{
					Key:         "diff",
					Title:       "Diff",
					Category:    "Utility",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "When changing (small) files and templates, show the differences in those files",
				},
				{
					Key:         "verbose",
					Title:       "Verbose",
					Category:    "Utility",
					Type:        "number",
					Default:     "0",
					Required:    false,
					Description: "Set the verbosity level. Default is 0",
				},
			},
		},
		Endpoint: models.Endpoint{},
	}

	return plugin, nil
}

// PluginRPCServer is the RPC server for Plugin
type PluginRPCServer struct {
	Impl plugins.Plugin
}

func (s *PluginRPCServer) ExecuteTask(request plugins.ExecuteTaskRequest, resp *plugins.Response) error {
	result, err := s.Impl.ExecuteTask(request)
	*resp = result
	return err
}

func (s *PluginRPCServer) CancelTask(request plugins.CancelTaskRequest, resp *plugins.Response) error {
	result, err := s.Impl.CancelTask(request)
	*resp = result
	return err
}

func (s *PluginRPCServer) EndpointRequest(request plugins.EndpointRequest, resp *plugins.Response) error {
	result, err := s.Impl.EndpointRequest(request)
	*resp = result
	return err
}

func (s *PluginRPCServer) Info(request plugins.InfoRequest, resp *models.Plugin) error {
	result, err := s.Impl.Info(request)
	*resp = result
	return err
}

// PluginServer is the implementation of plugin.Plugin interface
type PluginServer struct {
	Impl plugins.Plugin
}

func (p *PluginServer) Server(*plugin.MuxBroker) (interface{}, error) {
	return &PluginRPCServer{Impl: p.Impl}, nil
}

func (p *PluginServer) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &plugins.PluginRPC{Client: c}, nil
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "PLUGIN_MAGIC_COOKIE",
			MagicCookieValue: "hello",
		},
		Plugins: map[string]plugin.Plugin{
			"plugin": &PluginServer{Impl: &Plugin{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
