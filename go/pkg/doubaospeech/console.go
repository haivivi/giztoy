package doubaospeech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

const (
	consoleAPIURL     = "https://open.volcengineapi.com"
	consoleAPIVersion = "2024-01-01"
)

// consoleService 控制台服务实现
type consoleService struct {
	client *Client
	timbre *timbreService
	apiKey *apiKeyService
	svc    *serviceManageService
	mon    *monitoringService
	vc     *voiceCloneManageService
}

// newConsoleService 创建控制台服务
func newConsoleService(c *Client) iface.ConsoleService {
	cs := &consoleService{client: c}
	cs.timbre = &timbreService{console: cs}
	cs.apiKey = &apiKeyService{console: cs}
	cs.svc = &serviceManageService{console: cs}
	cs.mon = &monitoringService{console: cs}
	cs.vc = &voiceCloneManageService{console: cs}
	return cs
}

func (s *consoleService) Timbre() iface.TimbreService {
	return s.timbre
}

func (s *consoleService) APIKey() iface.APIKeyService {
	return s.apiKey
}

func (s *consoleService) Service() iface.ServiceManageService {
	return s.svc
}

func (s *consoleService) Monitoring() iface.MonitoringService {
	return s.mon
}

func (s *consoleService) VoiceCloneManage() iface.VoiceCloneManageService {
	return s.vc
}

// doConsoleRequest 发送控制台 API 请求
func (s *consoleService) doConsoleRequest(ctx context.Context, action string, params map[string]string, body interface{}, result interface{}) error {
	// 构建 URL
	u, _ := url.Parse(consoleAPIURL)
	q := u.Query()
	q.Set("Action", action)
	q.Set("Version", consoleAPIVersion)
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	method := http.MethodGet
	var bodyReader io.Reader
	if body != nil {
		method = http.MethodPost
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return wrapError(err, "marshal request body")
		}
		bodyReader = io.NopCloser(bytes.NewReader(jsonBytes))
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return wrapError(err, "create request")
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	s.client.setAuthHeaders(req)

	resp, err := s.client.config.httpClient.Do(req)
	if err != nil {
		return wrapError(err, "send request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return wrapError(err, "read response")
	}

	// 解析响应
	var apiResp struct {
		ResponseMetadata struct {
			RequestID string `json:"RequestId"`
			Action    string `json:"Action"`
			Version   string `json:"Version"`
			Error     *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error,omitempty"`
		} `json:"ResponseMetadata"`
		Result json.RawMessage `json:"Result"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return wrapError(err, "unmarshal response")
	}

	if apiResp.ResponseMetadata.Error != nil {
		return &Error{
			Message: apiResp.ResponseMetadata.Error.Message,
			TraceID: apiResp.ResponseMetadata.RequestID,
		}
	}

	if result != nil && apiResp.Result != nil {
		if err := json.Unmarshal(apiResp.Result, result); err != nil {
			return wrapError(err, "unmarshal result")
		}
	}

	return nil
}

// ================== 音色服务 ==================

type timbreService struct {
	console *consoleService
}

func (s *timbreService) ListBigModelTTSTimbres(ctx context.Context, req *iface.ListTimbresRequest) (*iface.ListTimbresResponse, error) {
	params := make(map[string]string)
	if req.PageNumber > 0 {
		params["PageNumber"] = fmt.Sprintf("%d", req.PageNumber)
	}
	if req.PageSize > 0 {
		params["PageSize"] = fmt.Sprintf("%d", req.PageSize)
	}
	if req.TimbreType != "" {
		params["TimbreType"] = req.TimbreType
	}

	var result struct {
		Total   int `json:"Total"`
		Timbres []struct {
			TimbreId    string `json:"TimbreId"`
			TimbreName  string `json:"TimbreName"`
			Language    string `json:"Language"`
			Gender      string `json:"Gender"`
			Description string `json:"Description,omitempty"`
		} `json:"Timbres"`
	}

	if err := s.console.doConsoleRequest(ctx, "ListBigModelTTSTimbres", params, nil, &result); err != nil {
		return nil, err
	}

	resp := &iface.ListTimbresResponse{
		Total:   result.Total,
		Timbres: make([]iface.TimbreInfo, len(result.Timbres)),
	}
	for i, t := range result.Timbres {
		resp.Timbres[i] = iface.TimbreInfo{
			TimbreId:    t.TimbreId,
			TimbreName:  t.TimbreName,
			Language:    t.Language,
			Gender:      t.Gender,
			Description: t.Description,
		}
	}

	return resp, nil
}

func (s *timbreService) ListSpeakers(ctx context.Context, req *iface.ListSpeakersRequest) (*iface.ListSpeakersResponse, error) {
	params := make(map[string]string)
	if req.PageNumber > 0 {
		params["PageNumber"] = fmt.Sprintf("%d", req.PageNumber)
	}
	if req.PageSize > 0 {
		params["PageSize"] = fmt.Sprintf("%d", req.PageSize)
	}
	if req.SpeakerType != "" {
		params["SpeakerType"] = req.SpeakerType
	}
	if req.Language != "" {
		params["Language"] = req.Language
	}

	var result struct {
		Total    int `json:"Total"`
		Speakers []struct {
			SpeakerId      string `json:"SpeakerId"`
			SpeakerName    string `json:"SpeakerName"`
			Language       string `json:"Language"`
			Gender         string `json:"Gender"`
			SampleAudioUrl string `json:"SampleAudioUrl,omitempty"`
		} `json:"Speakers"`
	}

	if err := s.console.doConsoleRequest(ctx, "ListSpeakers", params, nil, &result); err != nil {
		return nil, err
	}

	resp := &iface.ListSpeakersResponse{
		Total:    result.Total,
		Speakers: make([]iface.SpeakerInfo, len(result.Speakers)),
	}
	for i, sp := range result.Speakers {
		resp.Speakers[i] = iface.SpeakerInfo{
			SpeakerId:      sp.SpeakerId,
			SpeakerName:    sp.SpeakerName,
			Language:       sp.Language,
			Gender:         sp.Gender,
			SampleAudioUrl: sp.SampleAudioUrl,
		}
	}

	return resp, nil
}

// ================== API Key 服务 ==================

type apiKeyService struct {
	console *consoleService
}

func (s *apiKeyService) List(ctx context.Context) (*iface.ListAPIKeysResponse, error) {
	var result struct {
		APIKeys []struct {
			APIKeyId    string    `json:"APIKeyId"`
			Name        string    `json:"Name"`
			Status      string    `json:"Status"`
			Description string    `json:"Description,omitempty"`
			CreatedAt   time.Time `json:"CreatedAt"`
			ExpiredAt   time.Time `json:"ExpiredAt,omitempty"`
		} `json:"APIKeys"`
	}

	if err := s.console.doConsoleRequest(ctx, "ListAPIKeys", nil, nil, &result); err != nil {
		return nil, err
	}

	resp := &iface.ListAPIKeysResponse{
		APIKeys: make([]iface.APIKeyInfo, len(result.APIKeys)),
	}
	for i, k := range result.APIKeys {
		resp.APIKeys[i] = iface.APIKeyInfo{
			APIKeyId:    k.APIKeyId,
			Name:        k.Name,
			Status:      k.Status,
			Description: k.Description,
			CreatedAt:   k.CreatedAt,
			ExpiredAt:   k.ExpiredAt,
		}
	}

	return resp, nil
}

func (s *apiKeyService) Create(ctx context.Context, req *iface.CreateAPIKeyRequest) (*iface.CreateAPIKeyResponse, error) {
	body := map[string]interface{}{
		"Name": req.Name,
	}
	if !req.ExpiredAt.IsZero() {
		body["ExpiredAt"] = req.ExpiredAt.Format(time.RFC3339)
	}
	if req.Description != "" {
		body["Description"] = req.Description
	}

	var result struct {
		APIKeyId     string `json:"APIKeyId"`
		APIKeySecret string `json:"APIKeySecret"`
		Name         string `json:"Name"`
	}

	if err := s.console.doConsoleRequest(ctx, "CreateAPIKey", nil, body, &result); err != nil {
		return nil, err
	}

	return &iface.CreateAPIKeyResponse{
		APIKeyId:     result.APIKeyId,
		APIKeySecret: result.APIKeySecret,
		Name:         result.Name,
	}, nil
}

func (s *apiKeyService) Update(ctx context.Context, req *iface.UpdateAPIKeyRequest) error {
	body := map[string]interface{}{
		"APIKeyId": req.APIKeyId,
	}
	if req.Name != "" {
		body["Name"] = req.Name
	}
	if req.Status != "" {
		body["Status"] = req.Status
	}
	if !req.ExpiredAt.IsZero() {
		body["ExpiredAt"] = req.ExpiredAt.Format(time.RFC3339)
	}

	return s.console.doConsoleRequest(ctx, "UpdateAPIKey", nil, body, nil)
}

func (s *apiKeyService) Delete(ctx context.Context, apiKeyID string) error {
	body := map[string]interface{}{
		"APIKeyId": apiKeyID,
	}
	return s.console.doConsoleRequest(ctx, "DeleteAPIKey", nil, body, nil)
}

// ================== 服务管理 ==================

type serviceManageService struct {
	console *consoleService
}

func (s *serviceManageService) Status(ctx context.Context) (*iface.ServiceStatusResponse, error) {
	var result struct {
		Status      string    `json:"Status"`
		ActivatedAt time.Time `json:"ActivatedAt,omitempty"`
		Services    []struct {
			ServiceId   string `json:"ServiceId"`
			ServiceName string `json:"ServiceName"`
			Status      string `json:"Status"`
		} `json:"Services"`
	}

	if err := s.console.doConsoleRequest(ctx, "GetServiceStatus", nil, nil, &result); err != nil {
		return nil, err
	}

	resp := &iface.ServiceStatusResponse{
		Status:      iface.ServiceState(result.Status),
		ActivatedAt: result.ActivatedAt,
		Services:    make([]iface.ServiceInfo, len(result.Services)),
	}
	for i, svc := range result.Services {
		resp.Services[i] = iface.ServiceInfo{
			ServiceId:   svc.ServiceId,
			ServiceName: svc.ServiceName,
			Status:      iface.ServiceState(svc.Status),
		}
	}

	return resp, nil
}

func (s *serviceManageService) Activate(ctx context.Context, serviceID string) error {
	body := map[string]interface{}{"ServiceId": serviceID}
	return s.console.doConsoleRequest(ctx, "ActivateService", nil, body, nil)
}

func (s *serviceManageService) Pause(ctx context.Context, serviceID string) error {
	body := map[string]interface{}{"ServiceId": serviceID}
	return s.console.doConsoleRequest(ctx, "PauseService", nil, body, nil)
}

func (s *serviceManageService) Resume(ctx context.Context, serviceID string) error {
	body := map[string]interface{}{"ServiceId": serviceID}
	return s.console.doConsoleRequest(ctx, "ResumeService", nil, body, nil)
}

func (s *serviceManageService) Terminate(ctx context.Context, serviceID string) error {
	body := map[string]interface{}{"ServiceId": serviceID}
	return s.console.doConsoleRequest(ctx, "TerminateService", nil, body, nil)
}

// ================== 监控服务 ==================

type monitoringService struct {
	console *consoleService
}

func (s *monitoringService) GetQuota(ctx context.Context, serviceID string) (*iface.QuotaResponse, error) {
	params := map[string]string{"ServiceId": serviceID}
	var result iface.QuotaResponse
	if err := s.console.doConsoleRequest(ctx, "GetQuota", params, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *monitoringService) GetUsage(ctx context.Context, req *iface.UsageRequest) (*iface.UsageResponse, error) {
	params := map[string]string{
		"StartTime": req.StartTime.Format(time.RFC3339),
		"EndTime":   req.EndTime.Format(time.RFC3339),
	}
	if req.ServiceId != "" {
		params["ServiceId"] = req.ServiceId
	}
	if req.Granularity != "" {
		params["Granularity"] = string(req.Granularity)
	}

	var result iface.UsageResponse
	if err := s.console.doConsoleRequest(ctx, "GetUsage", params, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *monitoringService) GetQPS(ctx context.Context) (*iface.QPSResponse, error) {
	var result iface.QPSResponse
	if err := s.console.doConsoleRequest(ctx, "GetQPS", nil, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ================== 声音复刻管理 ==================

type voiceCloneManageService struct {
	console *consoleService
}

func (s *voiceCloneManageService) ListTrainStatus(ctx context.Context, speakerID string) (*iface.VoiceCloneTrainStatusResponse, error) {
	params := map[string]string{"SpeakerId": speakerID}
	var result iface.VoiceCloneTrainStatusResponse
	if err := s.console.doConsoleRequest(ctx, "GetVoiceCloneStatus", params, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *voiceCloneManageService) BatchListTrainStatus(ctx context.Context, req *iface.BatchListTrainStatusRequest) (*iface.BatchListTrainStatusResponse, error) {
	params := make(map[string]string)
	if req.PageNumber > 0 {
		params["PageNumber"] = fmt.Sprintf("%d", req.PageNumber)
	}
	if req.PageSize > 0 {
		params["PageSize"] = fmt.Sprintf("%d", req.PageSize)
	}
	if req.Status != "" {
		params["Status"] = string(req.Status)
	}

	var result iface.BatchListTrainStatusResponse
	if err := s.console.doConsoleRequest(ctx, "ListVoiceCloneStatus", params, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// 注册实现验证
var _ iface.ConsoleService = (*consoleService)(nil)
var _ iface.TimbreService = (*timbreService)(nil)
var _ iface.APIKeyService = (*apiKeyService)(nil)
var _ iface.ServiceManageService = (*serviceManageService)(nil)
var _ iface.MonitoringService = (*monitoringService)(nil)
var _ iface.VoiceCloneManageService = (*voiceCloneManageService)(nil)
