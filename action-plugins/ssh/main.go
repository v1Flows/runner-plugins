package main

import (
	"errors"
	"net/rpc"
	"strconv"
	"strings"
	"time"

	"github.com/melbahja/goph"
	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"
	"golang.org/x/crypto/ssh"

	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

// ifEmpty returns the defaultValue if the input is an empty string, otherwise it returns the input.
func ifEmpty(input, defaultValue string) string {
	if input == "" {
		return defaultValue
	}
	return input
}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "SSH",
				Lines: []models.Line{
					{
						Content: "Starting ssh action",
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

	var target string
	var port uint
	var username string
	var password string
	var privateKeyFile string
	var privateKeyFilePassword string
	var useSSHAgent bool
	var commands []string
	var sudo bool
	var sudoPassword string

	// access action params
	for _, param := range request.Step.Action.Params {
		if param.Key == "Target" {
			target = param.Value
		}
		if param.Key == "Port" {
			portInt, _ := strconv.ParseUint(param.Value, 10, 16)
			port = uint(portInt)
		}
		if param.Key == "Username" {
			username = param.Value
		}
		if param.Key == "Password" {
			password = param.Value
		}
		if param.Key == "PrivateKeyFile" {
			privateKeyFile = param.Value
		}
		if param.Key == "PrivateKeyFilePassword" {
			privateKeyFilePassword = param.Value
		}
		if param.Key == "UseSSHAgent" {
			useSSHAgent = strings.ToLower(param.Value) == "true"
		}
		if param.Key == "Commands" {
			commands = strings.Split(param.Value, "\n")
		}
		if param.Key == "Sudo" {
			sudo = strings.ToLower(param.Value) == "true"
		}
		if param.Key == "SudoPassword" {
			sudoPassword = param.Value
		}
	}

	var auth goph.Auth

	// use private key file if provided
	if privateKeyFile != "" {
		auth, err = goph.Key(privateKeyFile, ifEmpty(privateKeyFilePassword, ""))
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}

		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "SSH",
					Lines: []models.Line{
						{
							Content: "Use private key file to authenticate",
						},
					},
				},
			},
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
	}

	if useSSHAgent {
		auth, err = goph.UseAgent()
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}

		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "SSH",
					Lines: []models.Line{
						{
							Content: "Use SSH agent to authenticate",
						},
					},
				},
			},
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
	}

	// use password if provided
	if password != "" {
		auth = goph.Password(password)

		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "SSH",
					Lines: []models.Line{
						{
							Content: "Use password to authenticate",
						},
					},
				},
			},
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
	}

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "SSH",
				Lines: []models.Line{
					{
						Content: "Connecting to remote server " + target + " as " + username,
					},
				},
			},
		},
	}, request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	client, err := goph.NewConn(&goph.Config{
		User:     username,
		Addr:     target,
		Port:     port,
		Auth:     auth,
		Timeout:  goph.DefaultTimeout,
		Callback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "SSH",
					Lines: []models.Line{
						{
							Content: "Failed to connect to remote server",
							Color:   "danger",
						},
						{
							Content: err.Error(),
							Color:   "danger",
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

	// Defer closing the network connection.
	defer client.Close()

	for _, command := range commands {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "SSH",
					Lines: []models.Line{
						{
							Content: "-------------------------",
						},
						{
							Content: "Executing command: " + command,
						},
						{
							Content: "-------------------------",
						},
					},
				},
			},
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}

		// Execute your command.
		var out []byte
		if sudo {
			out, err = client.Run("echo " + sudoPassword + "| sudo -S " + command)
		} else {
			out, err = client.Run(command)
		}
		if err != nil {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "SSH",
						Lines: []models.Line{
							{
								Content: "Failed to execute command",
								Color:   "danger",
							},
							{
								Content: err.Error(),
								Color:   "danger",
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

		outputLines := strings.Split(string(out), "\n")
		for _, line := range outputLines {
			err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "SSH",
						Lines: []models.Line{
							{
								Content: line,
							},
						},
					},
				},
			}, request.Platform)
			if err != nil {
				return plugins.Response{
					Success: false,
				}, err
			}
		}
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
	}

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "SSH",
				Lines: []models.Line{
					{
						Content: "SSH action completed",
						Color:   "success",
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

func (p *Plugin) EndpointRequest(request plugins.EndpointRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, errors.New("not implemented")
}

func (p *Plugin) Info(request plugins.InfoRequest) (models.Plugin, error) {
	var plugin = models.Plugin{
		Name:    "SSH",
		Type:    "action",
		Version: "1.1.2",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "SSH",
			Description: "Connect to a remote server using SSH and execute commands",
			Plugin:      "ssh",
			Icon:        "hugeicons:server-stack-03",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "Target",
					Type:        "text",
					Default:     "",
					Required:    true,
					Description: "The target server IP address or hostname",
				},
				{
					Key:         "Port",
					Type:        "number",
					Default:     "22",
					Required:    true,
					Description: "The target server port",
				},
				{
					Key:         "Username",
					Type:        "text",
					Default:     "",
					Required:    true,
					Description: "The username to authenticate with",
				},
				{
					Key:         "Password",
					Type:        "password",
					Default:     "",
					Required:    false,
					Description: "The password to authenticate with",
				},
				{
					Key:         "PrivateKeyFile",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "The private key file path to authenticate with. This path must be accessible by the runner",
				},
				{
					Key:         "PrivateKeyFilePassword",
					Type:        "password",
					Default:     "",
					Required:    false,
					Description: "The password to decrypt the private key file",
				},
				{
					Key:         "Sudo",
					Type:        "boolean",
					Default:     "true",
					Required:    false,
					Description: "Use sudo to execute the commands",
				},
				{
					Key:         "SudoPassword",
					Type:        "password",
					Default:     "",
					Required:    false,
					Description: "The password to authenticate with sudo",
				},
				{
					Key:         "UseSSHAgent",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Use the SSH agent to authenticate",
				},
				{
					Key:         "Commands",
					Type:        "textarea",
					Default:     "",
					Required:    true,
					Description: "The commands to execute on the remote server. Each command should be on a new line",
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
