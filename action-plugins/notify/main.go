package main

import (
	"context"
	"errors"
	"net/rpc"
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

	// target := "www.alertflow.org"
	// count := 3
	// maxLostPackages := 0

	// for _, param := range request.Step.Action.Params {
	// 	if param.Key == "target" {
	// 		target = param.Value
	// 	}
	// 	if param.Key == "count" {
	// 		count, _ = strconv.Atoi(param.Value)
	// 	}
	// 	if param.Key == "maxLostPackages" {
	// 		maxLostPackages, _ = strconv.Atoi(param.Value)
	// 	}
	// }

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
				Title: "Notify",
				Lines: []models.Line{
					{
						Content:   "Notify Service",
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

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Notify",
				Lines: []models.Line{
					{
						Content:   "Notify completed successfully",
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
		Name:    "Notify",
		Type:    "action",
		Version: "1.0.0",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Notify",
			Description: "Send a notification",
			Plugin:      "notify",
			Icon:        "hugeicons:notification-01",
			Category:    "Notification",
			Params: []models.Params{
				{
					Key:         "service",
					Title:       "Service",
					Type:        "select",
					Default:     "microsoft_teams",
					Required:    true,
					Description: "Service to send the notification to",
					Category:    "General",
					Options: []models.Option{
						{
							Key:   "amazon_ses",
							Value: "Amazon SES",
						},
						{
							Key:   "amazon_sns",
							Value: "Amazon SNS",
						},
						{
							Key:   "bark",
							Value: "Bark",
						},
						{
							Key:   "dingtalk",
							Value: "DingTalk",
						},
						{
							Key:   "discord",
							Value: "Discord",
						},
						{
							Key:   "http",
							Value: "HTTP",
						},
						{
							Key:   "line",
							Value: "Line",
						},
						{
							Key:   "line_notify",
							Value: "Line Notify",
						},
						{
							Key:   "microsoft_teams",
							Value: "Microsoft Teams",
						},
						{
							Key:   "pagerduty",
							Value: "PagerDuty",
						},
						{
							Key:   "plivo",
							Value: "Plivo",
						},
						{
							Key:   "pushover",
							Value: "Pushover",
						},
						{
							Key:   "reddit",
							Value: "Reddit",
						},
						{
							Key:   "rocket_chat",
							Value: "Rocket.Chat",
						},
						{
							Key:   "sendgrid",
							Value: "SendGrid",
						},
						{
							Key:   "slack",
							Value: "Slack",
						},
						{
							Key:   "syslog",
							Value: "Syslog",
						},
						{
							Key:   "telegram",
							Value: "Telegram",
						},
						{
							Key:   "textmagic",
							Value: "TextMagic",
						},
						{
							Key:   "twilio",
							Value: "Twilio",
						},
						{
							Key:   "twitter",
							Value: "Twitter",
						},
						{
							Key:   "viber",
							Value: "Viber",
						},
						{
							Key:   "wechat",
							Value: "WeChat",
						},
						{
							Key:   "webpush_notification",
							Value: "Webpush Notification",
						},
					},
				},
				{
					Key:         "subject",
					Title:       "Subject",
					Type:        "text",
					Default:     "Notification",
					Required:    false,
					Description: "Subject of the notification",
					Category:    "General",
				},
				{
					Key:         "message",
					Title:       "Message",
					Type:        "text",
					Default:     "This is a notification",
					Required:    false,
					Description: "Message of the notification",
					Category:    "General",
				},
				{
					Key:         "amazon_ses_sender",
					Title:       "Sender Address",
					Type:        "text",
					Default:     "<sender@example.com>",
					Required:    false,
					Description: "Sender address for the notification",
					Category:    "Amazon SES",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "amazon_ses",
					},
				},
				{
					Key:         "amazon_ses_receivers",
					Title:       "Receiver Addresses",
					Type:        "text",
					Default:     "<receiver@example.com>,<receiver2@example.com>",
					Required:    false,
					Description: "Receiver addresses for the notification. Multiple addresses can be separated by commas.",
					Category:    "Amazon SES",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "amazon_ses",
					},
				},
				{
					Key:         "amazon_sns_queue_topics",
					Title:       "Queue Topics",
					Type:        "text",
					Default:     "<arn:aws:sns:us-east-1:123456789012:MyTopic1>,<arn:aws:sns:us-east-1:123456789012:MyTopic2>",
					Required:    false,
					Description: "SNS topic ARNs for the notification. Multiple ARNs can be separated by commas.",
					Category:    "Amazon SNS",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "amazon_sns",
					},
				},
				{
					Key:         "bark_device_key",
					Title:       "Bark Device Key",
					Type:        "text",
					Default:     "<your_bark_device_key>",
					Required:    false,
					Description: "Bark device key.",
					Category:    "Bark",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "bark",
					},
				},
				{
					Key:         "ding_talk_access_token",
					Title:       "DingTalk Access Token",
					Type:        "text",
					Default:     "<your_dingtalk_access_token>",
					Required:    false,
					Description: "DingTalk access token.",
					Category:    "DingTalk",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "dingtalk",
					},
				},
				{
					Key:         "ding_talk_secret",
					Title:       "DingTalk Secret",
					Type:        "text",
					Default:     "<your_dingtalk_secret>",
					Required:    false,
					Description: "DingTalk secret.",
					Category:    "DingTalk",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "dingtalk",
					},
				},
				{
					Key:         "discord_channel_id",
					Title:       "Discord Channel ID",
					Type:        "text",
					Default:     "<discord_channel_id1>,<discord_channel_id2>",
					Required:    false,
					Description: "Discord channel ID. Multiple IDs can be separated by commas.",
					Category:    "Discord",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "discord",
					},
				},
				{
					Key:         "http_url",
					Title:       "HTTP URL",
					Type:        "text",
					Default:     "<http://example.com>",
					Required:    false,
					Description: "HTTP URL. This will send a POST request to the specified URL.",
					Category:    "HTTP",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "http",
					},
				},
				{
					Key:         "line_channel_secret",
					Title:       "Line Channel Secret",
					Type:        "text",
					Default:     "<your_line_channel_secret>",
					Required:    false,
					Description: "Line channel secret.",
					Category:    "Line",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "line",
					},
				},
				{
					Key:         "line_channel_access_token",
					Title:       "Line Channel Access Token",
					Type:        "text",
					Default:     "<your_line_channel_access_token>",
					Required:    false,
					Description: "Line channel access token.",
					Category:    "Line",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "line",
					},
				},
				{
					Key:         "line_receivers",
					Title:       "Line Receivers",
					Type:        "text",
					Default:     "<userID1>,<groupID2>",
					Required:    false,
					Description: "Line receivers.",
					Category:    "Line",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "line",
					},
				},
				{
					Key:         "line_receivers_token",
					Title:       "Line Receivers Token",
					Type:        "text",
					Default:     "<receiverToken1>,<receiverToken2>",
					Required:    false,
					Description: "Line receiver tokens.",
					Category:    "Line Notify",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "line_notify",
					},
				},
				{
					Key:         "microsoft_teams_webhook_urls",
					Title:       "Microsoft Teams Webhook URLs",
					Type:        "text",
					Default:     "<https://webhook1.example.com>,<https://webhook2.example.com>",
					Required:    false,
					Description: "Microsoft Teams webhook URLs. Multiple URLs can be separated by commas.",
					Category:    "Microsoft Teams",
					DependsOn: models.DependsOn{
						Key:   "service",
						Value: "microsoft_teams",
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
