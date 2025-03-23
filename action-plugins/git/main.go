package main

import (
	"errors"
	"net/rpc"
	"os"
	"time"

	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"
	"github.com/v1Flows/shared-library/pkg/models"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/hashicorp/go-plugin"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	url := ""
	remoteName := ""
	branch := ""
	directory := ""
	username := ""
	password := ""
	token := ""
	privateKey := ""
	privateKeyPassphrase := ""

	// access action params
	for _, param := range request.Step.Action.Params {
		if param.Key == "url" {
			url = param.Value
		}
		if param.Key == "remote_name" {
			remoteName = param.Value
		}
		if param.Key == "branch" {
			branch = param.Value
		}
		if param.Key == "directory" {
			directory = request.Workspace + "/" + param.Value
		}
		if param.Key == "username" {
			username = param.Value
		}
		if param.Key == "password" {
			password = param.Value
		}
		if param.Key == "token" {
			token = param.Value
		}
		if param.Key == "private_key" {
			privateKey = param.Value
		}
		if param.Key == "private_key_passphrase" {
			privateKeyPassphrase = param.Value
		}
	}

	// update the step with the messages
	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Git",
				Lines: []models.Line{
					{
						Content: "Cloning repository " + url + " to " + directory,
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

	if privateKey == "" {
		// clone the repository with basic auth (username and password or token)
		_, err = git.PlainClone(directory, false, &git.CloneOptions{
			Auth: &http.BasicAuth{
				Username: func() string {
					if token != "" {
						return "abc123"
					}
					return username
				}(),
				Password: func() string {
					if token != "" {
						return token
					}
					return password
				}(),
			},
			URL:           url,
			Progress:      os.Stdout,
			RemoteName:    remoteName,
			ReferenceName: plumbing.ReferenceName(branch),
		})
		if err != nil {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Git",
						Lines: []models.Line{
							{
								Content: "Error cloning repository",
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
	} else {
		// clone the repository with ssh key

		// check if private key file exists
		if _, err := os.Stat(privateKey); os.IsNotExist(err) {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Git",
						Lines: []models.Line{
							{
								Content: "Error cloning repository",
								Color:   "danger",
							},
							{
								Content: "Private key file does not exist",
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
			}, errors.New("private key file does not exist")
		}

		publicKeys, err := ssh.NewPublicKeysFromFile("git", privateKey, privateKeyPassphrase)
		if err != nil {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Git",
						Lines: []models.Line{
							{
								Content: "Error cloning repository",
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

		_, err = git.PlainClone(directory, false, &git.CloneOptions{
			Auth:          publicKeys,
			URL:           url,
			Progress:      os.Stdout,
			RemoteName:    remoteName,
			ReferenceName: plumbing.ReferenceName(branch),
		})
		if err != nil {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Git",
						Lines: []models.Line{
							{
								Content: "Error cloning repository",
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
	}

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Git",
				Lines: []models.Line{
					{
						Content: "Repository cloned successfully",
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
		Name:    "Git",
		Type:    "action",
		Version: "1.0.0",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Git",
			Description: "Clone a repository",
			Plugin:      "git",
			Icon:        "mdi:git",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "url",
					Title:       "URL",
					Type:        "text",
					Default:     "",
					Required:    true,
					Description: "URL of the repository to clone",
					Category:    "Repository",
				},
				{
					Key:         "remote_name",
					Title:       "Remote Name",
					Type:        "text",
					Default:     "origin",
					Required:    true,
					Description: "Name of the remote to clone",
					Category:    "Repository",
				},
				{
					Key:         "branch",
					Title:       "Branch",
					Type:        "text",
					Default:     "main",
					Required:    true,
					Description: "Branch to clone",
					Category:    "Repository",
				},
				{
					Key:         "directory",
					Title:       "Directory",
					Type:        "text",
					Default:     "",
					Required:    true,
					Description: "Path to clone the repository to. The path prefix is the workspace directory: " + request.Workspace,
					Category:    "Repository",
				},
				{
					Key:         "username",
					Title:       "Username",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Username for authentication",
					Category:    "Authentication",
				},
				{
					Key:         "password",
					Title:       "Password",
					Type:        "password",
					Default:     "",
					Required:    false,
					Description: "Password for authentication",
					Category:    "Authentication",
				},
				{
					Key:         "token",
					Title:       "Token",
					Type:        "password",
					Default:     "",
					Required:    false,
					Description: "Token for authentication. If provided, username and password will be ignored",
					Category:    "Authentication",
				},
				{
					Key:         "private_key",
					Title:       "Private Key",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Private key for authentication",
					Category:    "Authentication",
				},
				{
					Key:         "private_key_passphrase",
					Title:       "Private Key Passphrase",
					Type:        "password",
					Default:     "",
					Required:    false,
					Description: "Passphrase for the private key",
					Category:    "Authentication",
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
