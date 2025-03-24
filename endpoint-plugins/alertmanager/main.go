// filepath: /path/to/ping-plugin/main.go
package main

import (
	"encoding/json"
	"fmt"
	"net/rpc"

	"github.com/google/uuid"
	"github.com/v1Flows/runner/pkg/alerts"
	"github.com/v1Flows/runner/pkg/flows"
	"github.com/v1Flows/runner/pkg/plugins"

	af_models "github.com/v1Flows/alertFlow/services/backend/pkg/models"
	shared_models "github.com/v1Flows/shared-library/pkg/models"

	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/tidwall/gjson"
)

type Payload struct {
	Receiver string `json:"receiver"`
	Status   string `json:"status"`
}

func parseTime(timeStr string) time.Time {
	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}
	}
	return parsedTime
}

// AlertmanagerEndpointPlugin is an implementation of the Plugin interface
type AlertmanagerEndpointPlugin struct{}

func (p *AlertmanagerEndpointPlugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, nil
}

func (p *AlertmanagerEndpointPlugin) EndpointRequest(request plugins.EndpointRequest) (plugins.Response, error) {
	if request.Platform != "alertflow" {
		return plugins.Response{
			Success: false,
		}, fmt.Errorf("platform not supported")
	}

	if request.Body == nil {
		return plugins.Response{
			Success: false,
		}, fmt.Errorf("no body found")
	}

	incPayload := request.Body
	payloadString := string(incPayload)
	payload := Payload{}
	json.Unmarshal(incPayload, &payload)

	alertData := af_models.Alerts{
		Payload:  incPayload,
		FlowID:   payload.Receiver,
		RunnerID: request.Config.Alertflow.RunnerID,
		Plugin:   "Alertmanager",
		Status:   payload.Status,
	}

	// search for alertname in payload
	if gjson.Get(payloadString, "commonLabels.alertname").Exists() {
		alertData.Name = gjson.Get(payloadString, "commonLabels.alertname").String()
	} else if gjson.Get(payloadString, "groupLabels.alertname").Exists() {
		alertData.Name = gjson.Get(payloadString, "groupLabels.alertname").String()
	} else {
		alertData.Name = "Unknown"
	}

	// get sub alerts
	if gjson.Get(payloadString, "alerts").Exists() {
		for _, alert := range gjson.Get(payloadString, "alerts").Array() {
			alertData.SubAlerts = append(alertData.SubAlerts, af_models.SubAlerts{
				ID:         uuid.New().String(),
				Name:       alert.Get("labels.alertname").String(),
				Status:     alert.Get("status").String(),
				Labels:     json.RawMessage(alert.Get("labels").Raw),
				StartedAt:  parseTime(alert.Get("startsAt").String()),
				ResolvedAt: parseTime(alert.Get("endsAt").String()),
			})
		}
	}

	// get flow data
	bytes, err := flows.GetFlowData(request.Config, payload.Receiver, request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	if bytes == nil {
		return plugins.Response{
			Success: false,
		}, fmt.Errorf("flow not found")
	}

	flow := af_models.Flows{}
	err = json.Unmarshal(bytes, &flow)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	if flow.GroupAlerts {
		// check if payload matched the group key identifier
		if gjson.Get(payloadString, flow.GroupAlertsIdentifier).Exists() {
			alertData.GroupKey = flow.GroupAlertsIdentifier + "=" + gjson.Get(payloadString, flow.GroupAlertsIdentifier).String()

			// get grouped alerts
			groupedAlerts, err := alerts.GetGroupedAlerts(request.Config, payload.Receiver, alertData.GroupKey)
			if err != nil {
				return plugins.Response{
					Success: false,
				}, err
			}

			if len(groupedAlerts) > 0 {
				// get the first alert in the group
				alertData.ParentID = groupedAlerts[0].ID.String()
			}
		}
	}

	// check if alert is resolved
	alerts.SendAlert(request.Config, alertData)

	return plugins.Response{
		Success: true,
	}, nil
}

func (p *AlertmanagerEndpointPlugin) Info(request plugins.InfoRequest) (shared_models.Plugin, error) {
	return shared_models.Plugin{
		Name:    "Alertmanager",
		Type:    "endpoint",
		Version: "1.2.2",
		Author:  "JustNZ",
		Endpoint: shared_models.Endpoint{
			ID:    "alertmanager",
			Name:  "Alertmanager",
			Path:  "/alertmanager",
			Icon:  "vscode-icons:file-type-prometheus",
			Color: "#e6522c",
		},
	}, nil
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

func (s *PluginRPCServer) Info(request plugins.InfoRequest, resp *shared_models.Plugin) error {
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
			"plugin": &PluginServer{Impl: &AlertmanagerEndpointPlugin{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
