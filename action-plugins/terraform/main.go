package main

import (
	"context"
	"errors"
	"net/rpc"
	"strconv"
	"time"

	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"

	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-exec/tfexec"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	tf_version := ""
	workdir := ""
	init := false
	plan := false
	plan_output := ""
	apply := false

	for _, param := range request.Step.Action.Params {
		if param.Key == "tf_version" {
			tf_version = param.Value
		}
		if param.Key == "workdir" {
			workdir = param.Value
		}
		if param.Key == "init" {
			init, _ = strconv.ParseBool(param.Value)
		}
		if param.Key == "plan" {
			plan, _ = strconv.ParseBool(param.Value)
		}
		if param.Key == "plan_output" {
			plan_output = param.Value
		}
		if param.Key == "apply" {
			apply, _ = strconv.ParseBool(param.Value)
		}
	}

	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Terraform",
				Lines: []models.Line{
					{
						Content: "Terraform Action started",
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

	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion(tf_version)),
	}

	execPath, err := installer.Install(context.Background())
	if err != nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Terraform",
					Lines: []models.Line{
						{
							Content: "Terraform Install failed",
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

	tf, err := tfexec.NewTerraform(workdir, execPath)
	if err != nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Terraform",
					Lines: []models.Line{
						{
							Content: "Terraform NewTerraform failed",
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

	if init {
		err = tf.Init(context.Background(), tfexec.Upgrade(true))
		if err != nil {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Terraform",
						Lines: []models.Line{
							{
								Content: "Terraform Init failed",
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

	if plan {
		diff, err := tf.Plan(context.Background(), tfexec.Out(plan_output))
		if err != nil {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Terraform",
						Lines: []models.Line{
							{
								Content: "Terraform Plan failed",
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

		if diff {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Terraform",
						Lines: []models.Line{
							{
								Content: "Terraform Plan has changes",
								Color:   "warning",
							},
						},
					},
				},
				Status: "running",
			}, request.Platform)
			if err != nil {
				return plugins.Response{
					Success: false,
				}, err
			}
		} else {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Terraform",
						Lines: []models.Line{
							{
								Content: "Terraform Plan has no changes",
								Color:   "success",
							},
						},
					},
				},
				Status: "running",
			}, request.Platform)
			if err != nil {
				return plugins.Response{
					Success: false,
				}, err
			}
		}
	}

	if apply {
		// Fail if no plan_output is specified
		if plan_output == "" {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Terraform",
						Lines: []models.Line{
							{
								Content: "Terraform Apply requires a plan_output file",
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

		err = tf.Apply(context.Background(), tfexec.DirOrPlan(plan_output))
		if err != nil {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Terraform",
						Lines: []models.Line{
							{
								Content: "Terraform Apply failed",
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
		Name:    "Terraform",
		Type:    "action",
		Version: "1.0.0",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Terraform",
			Description: "Init, Plan, Apply, Destroy Terraform",
			Plugin:      "terraform",
			Icon:        "logos:terraform-icon",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "tf_version",
					Title:       "Terraform Version",
					Type:        "string",
					Default:     "1.0.6",
					Required:    true,
					Description: "Terraform Version to use",
				},
				{
					Key:         "workdir",
					Title:       "Working Directory",
					Type:        "string",
					Default:     "",
					Required:    true,
					Description: "Working directory where terraform files are located. Default prefix is the runner workspace",
				},
				{
					Key:         "init",
					Title:       "Initialize",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Perform an terraform init",
				},
				{
					Key:         "plan",
					Title:       "Plan",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Perform an terraform plan",
				},
				{
					Key:         "plan_output",
					Title:       "Plan Output",
					Type:        "string",
					Default:     "",
					Required:    false,
					Description: "Output the terraform plan to a file",
				},
				{
					Key:         "Apply",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Perform an terraform apply. Requires a plan_output file",
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
