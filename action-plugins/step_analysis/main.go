package main

import (
	"context"
	"errors"
	"net/rpc"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"

	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

var (
	taskCancels   = make(map[string]context.CancelFunc)
	taskCancelsMu sync.Mutex
)

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

	actionID := ""
	endsWith := ""
	regex := false
	regexPattern := ""
	lineContent := ""
	condition := ""

	for _, param := range request.Step.Action.Params {
		if param.Key == "actionID" {
			actionID = param.Value
		}
		if param.Key == "endsWith" {
			endsWith = param.Value
		}
		if param.Key == "regex" {
			regex, _ = strconv.ParseBool(param.Value)
		}
		if param.Key == "regexPattern" {
			regexPattern = param.Value
		}
		if param.Key == "lineContent" {
			lineContent = param.Value
		}
		if param.Key == "condition" {
			condition = param.Value
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
				Title: "Step Analysis",
				Lines: []models.Line{
					{
						Content:   "Starting step analysis",
						Timestamp: time.Now(),
					},
					{
						Content:   "Requesting all steps for the current execution",
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

	// request all steps
	steps, err := executions.GetSteps(request.Config, request.Execution.ID.String(), request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Step Analysis",
				Lines: []models.Line{
					{
						Content:   "Received " + strconv.Itoa(len(steps)) + " total steps for further analysis",
						Timestamp: time.Now(),
					},
					{
						Content:   "Filtering steps based on the provided action ID: " + actionID,
						Timestamp: time.Now(),
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

	// filter steps based on the provided step ID
	var targetStep models.ExecutionSteps
	for _, step := range steps {
		if step.Action.ID.String() == actionID {
			targetStep = step
			break
		}
	}

	if targetStep.Action.Name == "" {
		err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Step Analysis",
					Lines: []models.Line{
						{
							Content:   "Step not found",
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
		}, nil
	}

	var stepName string
	if targetStep.Action.CustomName != "" {
		stepName = targetStep.Action.CustomName
	} else {
		stepName = targetStep.Action.Name
	}

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Step Analysis",
				Lines: []models.Line{
					{
						Content:   "Step found: " + stepName,
						Color:     "success",
						Timestamp: time.Now(),
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

	// fail if step is in pending state
	if targetStep.Status == "pending" {
		err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Step Analysis",
					Lines: []models.Line{
						{
							Content:   "Step is in pending state. Please select an step that has finished",
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
		}, nil
	}

	// regex is false: check the step messages based on the provided string and condition
	if !regex {
		err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Step Analysis",
					Lines: []models.Line{
						{
							Content:   "Checking the step messages based on the provided string and condition",
							Timestamp: time.Now(),
						},
						{
							Content:   "Line content: " + lineContent,
							Timestamp: time.Now(),
						},
						{
							Content:   "Condition: " + condition,
							Timestamp: time.Now(),
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

		if condition == "match" {
			var found bool
			var discoveredLineContent string

			for _, message := range targetStep.Messages {
				for _, line := range message.Lines {
					if line.Content == lineContent {
						found = true
						discoveredLineContent = line.Content
						break
					}
				}
			}
			if found {
				err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
					ID: request.Step.ID,
					Messages: []models.Message{
						{
							Title: "Step Analysis",
							Lines: []models.Line{
								{
									Content:   "Line in step message found",
									Color:     "success",
									Timestamp: time.Now(),
								},
								{
									Content:   "Matched line content: " + discoveredLineContent,
									Timestamp: time.Now(),
								},
							},
						},
					},
					Status:     endsWith,
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
			} else {
				err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
					ID: request.Step.ID,
					Messages: []models.Message{
						{
							Title: "Step Analysis",
							Lines: []models.Line{
								{
									Content:   "Line not found in step messages",
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
				}, nil
			}

		} else if condition == "notMatch" {
			var found bool
			var discoveredLineContent string

			for _, message := range targetStep.Messages {
				for _, line := range message.Lines {
					if line.Content == lineContent {
						found = true
						discoveredLineContent = line.Content
						break
					}
				}
			}

			if !found {
				err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
					ID: request.Step.ID,
					Messages: []models.Message{
						{
							Title: "Step Analysis",
							Lines: []models.Line{
								{
									Content:   "Did not find the line in step messages",
									Color:     "success",
									Timestamp: time.Now(),
								},
							},
						},
					},
					Status:     endsWith,
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
			} else {
				err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
					ID: request.Step.ID,
					Messages: []models.Message{
						{
							Title: "Step Analysis",
							Lines: []models.Line{
								{
									Content:   "Line was found in step messages",
									Color:     "danger",
									Timestamp: time.Now(),
								},
								{
									Content:   "Matched line content: " + discoveredLineContent,
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
				}, nil
			}
		} else if condition == "contains" {
			for _, message := range targetStep.Messages {
				for _, line := range message.Lines {
					if strings.Contains(line.Content, lineContent) {
						err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
							ID: request.Step.ID,
							Messages: []models.Message{
								{
									Title: "Step Analysis",
									Lines: []models.Line{
										{
											Content:   "Lines in step message contains the string",
											Color:     "success",
											Timestamp: time.Now(),
										},
										{
											Content:   "Matched line content: " + line.Content,
											Timestamp: time.Now(),
										},
									},
								},
							},
							Status:     endsWith,
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
				}
			}

			err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Step Analysis",
						Lines: []models.Line{
							{
								Content:   "Lines in step message does not contain the string",
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
			}, nil

		} else if condition == "notContains" {
			var found bool
			var discoveredLineContent string

			for _, message := range targetStep.Messages {
				for _, line := range message.Lines {
					if strings.Contains(line.Content, lineContent) {
						found = true
						discoveredLineContent = line.Content
						break
					}
				}
			}

			if !found {
				err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
					ID: request.Step.ID,
					Messages: []models.Message{
						{
							Title: "Step Analysis",
							Lines: []models.Line{
								{
									Content:   "Lines in step message does not contain the string",
									Color:     "success",
									Timestamp: time.Now(),
								},
							},
						},
					},
					Status:     endsWith,
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
			} else {
				err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
					ID: request.Step.ID,
					Messages: []models.Message{
						{
							Title: "Step Analysis",
							Lines: []models.Line{
								{
									Content:   "Lines in step message contains the string",
									Color:     "danger",
									Timestamp: time.Now(),
								},
								{
									Content:   "Matched line content: " + discoveredLineContent,
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
				}, nil
			}
		}
	} else {
		err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Step Analysis",
					Lines: []models.Line{
						{
							Content:   "Checking the step messages based on the provided regex pattern",
							Timestamp: time.Now(),
						},
						{
							Content:   "Regex Pattern: " + regexPattern,
							Timestamp: time.Now(),
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

		// check if the regex pattern matches any of the step messages
		var found bool
		var discoveredLineContent string
		for _, message := range targetStep.Messages {
			for _, line := range message.Lines {
				match, _ := regexp.MatchString(regexPattern, line.Content)

				if match {
					found = true
					discoveredLineContent = line.Content
					break
				}
			}
		}

		if found {
			err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Step Analysis",
						Lines: []models.Line{
							{
								Content:   "Regex pattern matched in step messages",
								Color:     "success",
								Timestamp: time.Now(),
							},
							{
								Content:   "Matched line content: " + discoveredLineContent,
								Timestamp: time.Now(),
							},
						},
					},
				},
				Status:     endsWith,
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
		} else {
			err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Step Analysis",
						Lines: []models.Line{
							{
								Content:   "Regex pattern did not match in step messages",
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
			}, nil
		}
	}

	return plugins.Response{
		Success: false,
	}, errors.New("something went wrong")
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
		Name:    "Step Analysis",
		Type:    "action",
		Version: "1.2.2",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Step Analysis",
			Description: "Analyse the previous steps and their messages",
			Plugin:      "step_analysis",
			Icon:        "hugeicons:package-search",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "actionID",
					Title:       "Action ID",
					Description: "ID of the action to analyse",
					Type:        "text",
					Default:     "",
					Required:    true,
					Category:    "General",
				},
				{
					Key:         "endsWith",
					Title:       "Ends With",
					Description: "Choose in which status the step should end if the message matches",
					Type:        "select",
					Default:     "success",
					Required:    true,
					Category:    "General",
					Options: []models.Option{
						{
							Key:   "success",
							Value: "Success",
						},
						{
							Key:   "error",
							Value: "Error",
						},
						{
							Key:   "warning",
							Value: "Warning",
						},
						{
							Key:   "noPatternMatch",
							Value: "No Pattern Match",
						},
					},
				},
				{
					Key:         "regex",
					Title:       "Regex",
					Description: "Regex to match the messages",
					Type:        "boolean",
					Default:     "false",
					Required:    true,
					Category:    "Regex",
				},
				{
					Key:         "regexPattern",
					Title:       "Regex Pattern",
					Description: "Regex pattern to match the messages",
					Type:        "text",
					Default:     "",
					Required:    false,
					Category:    "Regex",
				},
				{
					Key:         "lineContent",
					Title:       "Line Content",
					Description: "Content of the line to check for. If regex is true, this will be ignored",
					Type:        "text",
					Default:     "",
					Required:    false,
					Category:    "Line Check",
				},
				{
					Key:         "condition",
					Title:       "Condition",
					Description: "Condition to match for the message lines. If regex is true, this will be ignored",
					Type:        "select",
					Default:     "match",
					Required:    true,
					Category:    "Line Check",
					Options: []models.Option{
						{
							Key:   "match",
							Value: "Match",
						},
						{
							Key:   "notMatch",
							Value: "Not Match",
						},
						{
							Key:   "contains",
							Value: "Contains",
						},
						{
							Key:   "notContains",
							Value: "Not Contains",
						},
					},
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
