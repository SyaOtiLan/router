package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/yeying-community/router/common/random"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/gorm"
)

const (
	ChannelStatusUnknown          = 0
	ChannelStatusEnabled          = 1 // don't use 0, 0 is the default value!
	ChannelStatusManuallyDisabled = 2 // also don't use 0
	ChannelStatusAutoDisabled     = 3
	ChannelStatusCreating         = 4
	ChannelStatusHalfOpen         = 5

	ChannelIdentifierMaxLength = 64
	ChannelHalfOpenPriority    = -1 << 60
)

var channelIdentifierPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

type PlatformChannel struct {
	Id                    string         `json:"id" gorm:"type:char(36);primaryKey"`
	Protocol              string         `json:"protocol" gorm:"type:varchar(64);default:'openai';index"`
	Key                   string         `json:"key" gorm:"type:text"`
	Status                int            `json:"status" gorm:"default:1;index"`
	Name                  string         `json:"name" gorm:"type:varchar(64);not null;uniqueIndex"`
	Weight                *uint          `json:"weight" gorm:"default:0"`
	CreatedTime           int64          `json:"created_time" gorm:"bigint;index"`
	UpdatedAt             int64          `json:"updated_at" gorm:"bigint;index"`
	TestTime              int64          `json:"test_time" gorm:"bigint"`
	ResponseTime          int            `json:"response_time"`
	BaseURL               *string        `json:"base_url" gorm:"column:base_url;default:''"`
	Other                 *string        `json:"other"`
	Models                string         `json:"models" gorm:"-"`
	AvailableModels       []string       `json:"available_models,omitempty" gorm:"-"`
	ChannelModels         []ChannelModel `json:"channel_models,omitempty" gorm:"-"`
	Tests                 []ChannelTest  `json:"channel_tests,omitempty" gorm:"-"`
	TestsLastTestedAt     int64          `json:"channel_tests_last_tested_at,omitempty" gorm:"-"`
	UsedQuota             int64          `json:"used_quota" gorm:"bigint;default:0"`
	Priority              *int64         `json:"priority" gorm:"bigint;default:0"`
	Config                string         `json:"config"`
	SystemPrompt          *string        `json:"system_prompt" gorm:"type:text"`
	TestModel             string         `json:"test_model" gorm:"type:varchar(255);default:''"`
	KeySet                bool           `json:"key_set" gorm:"-"`
	KeyPreview            string         `json:"key_preview,omitempty" gorm:"-"`
	ModelsProvided        bool           `json:"-" gorm:"-"`
	ChannelModelsProvided bool           `json:"-" gorm:"-"`
	NameProvided          bool           `json:"-" gorm:"-"`
}

// ChannelCandidate is the shared runtime view used by routing. It embeds
// platform channel capabilities while keeping source and owner metadata out of
// both persistence tables and public API payloads.
type ChannelCandidate struct {
	PlatformChannel     `gorm:"embedded"`
	PersonalChannelName string `json:"-" gorm:"-"`
	SourceType          string `json:"-" gorm:"-"`
	OwnerID             string `json:"-" gorm:"-"`
}

const (
	ChannelCandidateSourcePlatform  = "platform"
	ChannelCandidateSourcePersonal  = "personal"
	ChannelCandidateSourcePublisher = "publisher"
)

func (PlatformChannel) TableName() string  { return "platform_channels" }
func (ChannelCandidate) TableName() string { return "platform_channels" }

func (candidate *ChannelCandidate) AfterFind(tx *gorm.DB) error {
	if err := candidate.PlatformChannel.AfterFind(tx); err != nil {
		return err
	}
	candidate.SourceType = ChannelCandidateSourcePlatform
	return nil
}

func NewPlatformChannelCandidate(channel *PlatformChannel) *ChannelCandidate {
	if channel == nil {
		return nil
	}
	return &ChannelCandidate{PlatformChannel: *channel, SourceType: ChannelCandidateSourcePlatform}
}

type ChannelConfig struct {
	Region            string `json:"region,omitempty"`
	SK                string `json:"sk,omitempty"`
	AK                string `json:"ak,omitempty"`
	AppID             string `json:"app_id,omitempty"`
	UserID            string `json:"user_id,omitempty"`
	ResourceID        string `json:"resource_id,omitempty"`
	APIVersion        string `json:"api_version,omitempty"`
	LibraryID         string `json:"library_id,omitempty"`
	Plugin            string `json:"plugin,omitempty"`
	APIBaseURL        string `json:"api_base_url,omitempty"`
	VertexAIProjectID string `json:"vertex_ai_project_id,omitempty"`
	VertexAIADC       string `json:"vertex_ai_adc,omitempty"`
}

func normalizeConfiguredBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func baseURLHasVersionSuffix(raw string, suffix string) bool {
	normalized := strings.ToLower(normalizeConfiguredBaseURL(raw))
	return normalized != "" && strings.HasSuffix(normalized, strings.ToLower(strings.TrimSpace(suffix)))
}

func (config ChannelConfig) GetAPIBaseURL() string {
	return normalizeConfiguredBaseURL(config.APIBaseURL)
}

func (channel *PlatformChannel) NormalizeProtocol() {
	if channel == nil {
		return
	}
	protocol := relaychannel.NormalizeProtocolName(channel.Protocol)
	if protocol == "" {
		protocol = "openai"
	}
	channel.Protocol = protocol
}

func NormalizeChannelIdentifier(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func ValidateChannelIdentifier(id string) error {
	normalized := NormalizeChannelIdentifier(id)
	switch {
	case normalized == "":
		return fmt.Errorf("渠道标识不能为空")
	case len(normalized) > ChannelIdentifierMaxLength:
		return fmt.Errorf("渠道标识长度不能超过 %d 个字符", ChannelIdentifierMaxLength)
	case !channelIdentifierPattern.MatchString(normalized):
		return fmt.Errorf("渠道标识只支持小写字母、数字和 -")
	default:
		return nil
	}
}

func (channel *PlatformChannel) NormalizeIdentity() {
	if channel == nil {
		return
	}
	channel.Id = strings.TrimSpace(channel.Id)
	channel.Name = NormalizeChannelIdentifier(channel.Name)
}

func (channel *PlatformChannel) AfterFind(tx *gorm.DB) error {
	channel.NormalizeIdentity()
	channel.NormalizeProtocol()
	return nil
}

func (channel *PlatformChannel) ValidateIdentifier() error {
	if channel == nil {
		return fmt.Errorf("渠道不能为空")
	}
	return ValidateChannelIdentifier(channel.Name)
}

func (channel *PlatformChannel) ValidateProtocolConfiguration() error {
	if channel == nil {
		return fmt.Errorf("渠道不能为空")
	}
	switch channel.GetChannelProtocol() {
	case relaychannel.DeepSeek:
		if baseURLHasVersionSuffix(channel.GetBaseURL(), "/v1") {
			return fmt.Errorf("DeepSeek 渠道 base_url 不能追加 /v1，请使用 https://api.deepseek.com 或 https://api.deepseek.com/beta")
		}
		cfg, err := channel.LoadConfig()
		if err != nil {
			return nil
		}
		if baseURLHasVersionSuffix(cfg.GetAPIBaseURL(), "/v1") {
			return fmt.Errorf("DeepSeek 渠道 config.api_base_url 不能追加 /v1，请使用 https://api.deepseek.com 或 https://api.deepseek.com/beta")
		}
	case relaychannel.VolcEngine:
		if !channel.UsesVolcengineRealtimeEndpoint() {
			return nil
		}
		cfg, err := channel.LoadConfig()
		if err != nil {
			return nil
		}
		if strings.TrimSpace(cfg.AppID) == "" {
			return fmt.Errorf("Volcengine Realtime 渠道 config.app_id 不能为空")
		}
	}
	return nil
}

func (channel *PlatformChannel) UsesVolcengineRealtimeEndpoint() bool {
	if channel == nil {
		return false
	}
	if channel.GetChannelProtocol() != relaychannel.VolcEngine {
		return false
	}
	for _, row := range channel.GetChannelModels() {
		if !row.Selected {
			continue
		}
		if NormalizeRequestedChannelModelEndpoint(row.Endpoint) == ChannelModelEndpointRealtime {
			return true
		}
		for _, endpoint := range row.Endpoints {
			if NormalizeRequestedChannelModelEndpoint(endpoint) == ChannelModelEndpointRealtime {
				return true
			}
		}
	}
	return false
}

func (channel *PlatformChannel) DisplayName() string {
	if channel == nil {
		return ""
	}
	if name := NormalizeChannelIdentifier(channel.Name); name != "" {
		return name
	}
	return strings.TrimSpace(channel.Id)
}

func (channel *PlatformChannel) EnsureID() {
	if channel == nil {
		return
	}
	if strings.TrimSpace(channel.Id) == "" {
		channel.Id = random.GetUUID()
	}
}

func (channel *PlatformChannel) GetProtocol() string {
	if channel == nil {
		return "openai"
	}
	protocol := relaychannel.NormalizeProtocolName(channel.Protocol)
	if protocol != "" {
		return protocol
	}
	return "openai"
}

func (channel *PlatformChannel) GetChannelProtocol() int {
	if channel == nil {
		return relaychannel.OpenAI
	}
	return relaychannel.TypeByProtocol(channel.GetProtocol())
}

func GetChannelById(id string) (*PlatformChannel, error) {
	return mustChannelRepo().GetChannelById(id)
}

func (channel *PlatformChannel) GetPriority() int64 {
	if channel.Priority == nil {
		return 0
	}
	return *channel.Priority
}

func (channel *PlatformChannel) GetWeight() uint {
	if channel == nil || channel.Weight == nil || *channel.Weight == 0 {
		return 1
	}
	return *channel.Weight
}

func CloneChannelWithPriority(channel *ChannelCandidate, priority int64) *ChannelCandidate {
	if channel == nil {
		return nil
	}
	cloned := *channel
	clonedPriority := priority
	cloned.PlatformChannel.Priority = &clonedPriority
	return &cloned
}

func (channel *PlatformChannel) GetBaseURL() string {
	if channel.BaseURL == nil {
		return ""
	}
	return strings.TrimSpace(*channel.BaseURL)
}

func (channel *PlatformChannel) ResolveAPIBaseURL(requestPath string) string {
	return channel.ResolveAPIBaseURLForModel(requestPath)
}

func (channel *PlatformChannel) ResolveAPIBaseURLForModel(requestPath string, modelCandidates ...string) string {
	if channel == nil {
		return ""
	}
	if policyBaseURL := CacheGetChannelModelEndpointAccessPolicyBaseURL(strings.TrimSpace(channel.Id), requestPath, modelCandidates...); policyBaseURL != "" {
		return policyBaseURL
	}
	if endpointBaseURL := CacheGetChannelModelEndpointBaseURL(strings.TrimSpace(channel.Id), requestPath, modelCandidates...); endpointBaseURL != "" {
		return endpointBaseURL
	}
	if cfg, err := channel.LoadConfig(); err == nil {
		if apiBaseURL := cfg.GetAPIBaseURL(); apiBaseURL != "" {
			return apiBaseURL
		}
	}
	if baseURL := normalizeConfiguredBaseURL(channel.GetBaseURL()); baseURL != "" {
		return baseURL
	}
	return normalizeConfiguredBaseURL(relaychannel.BaseURLByProtocol(channel.GetProtocol()))
}

func (channel *PlatformChannel) GetModelMapping() map[string]string {
	selected := channel.selectedChannelModels()
	if len(selected) == 0 {
		return nil
	}
	modelMapping := make(map[string]string, len(selected))
	for _, row := range selected {
		if row.Model == "" || row.UpstreamModel == "" || row.UpstreamModel == row.Model {
			continue
		}
		modelMapping[row.Model] = row.UpstreamModel
	}
	if len(modelMapping) == 0 {
		return nil
	}
	return modelMapping
}

func (channel *PlatformChannel) GetChannelModels() []ChannelModel {
	if channel == nil {
		return nil
	}
	if len(channel.ChannelModels) > 0 {
		rows := NormalizeChannelModelsPreserveOrder(channel.ChannelModels)
		for i := range rows {
			completeChannelModelRowDefaults(&rows[i], channel.GetChannelProtocol())
		}
		return rows
	}
	selected := ParseChannelModelCSV(channel.Models)
	if len(selected) == 0 {
		return []ChannelModel{}
	}
	return BuildDefaultChannelModelsWithProtocol(selected, channel.GetChannelProtocol())
}

func (channel *PlatformChannel) SelectedModelIDs() []string {
	if channel == nil {
		return nil
	}
	if len(channel.ChannelModels) > 0 {
		modelIDs := make([]string, 0, len(channel.ChannelModels))
		for _, row := range channel.GetChannelModels() {
			if !row.Selected {
				continue
			}
			modelIDs = append(modelIDs, row.Model)
		}
		return NormalizeChannelModelIDsPreserveOrder(modelIDs)
	}
	return ParseChannelModelCSV(channel.Models)
}

func (channel *PlatformChannel) SetSelectedModelIDs(modelIDs []string) {
	if channel == nil {
		return
	}
	normalized := NormalizeChannelModelIDsPreserveOrder(modelIDs)
	channel.Models = JoinChannelModelCSV(normalized)
	if len(channel.ChannelModels) == 0 {
		return
	}

	selectedSet := buildChannelModelSelectionSet(normalized)
	existing := NormalizeChannelModelsPreserveOrder(channel.ChannelModels)
	next := make([]ChannelModel, 0, len(existing)+len(normalized))
	seen := make(map[string]struct{}, len(existing)+len(normalized))
	for _, row := range existing {
		if row.Model == "" {
			continue
		}
		row.Selected = false
		if _, ok := selectedSet[row.Model]; ok {
			row.Selected = true
		}
		completeChannelModelRowDefaults(&row, channel.GetChannelProtocol())
		next = append(next, row)
		seen[row.Model] = struct{}{}
	}
	for _, modelID := range normalized {
		if _, ok := seen[modelID]; ok {
			continue
		}
		rows := BuildDefaultChannelModelsWithProtocol([]string{modelID}, channel.GetChannelProtocol())
		if len(rows) == 0 {
			continue
		}
		next = append(next, rows[0])
	}
	channel.SetChannelModels(next)
}

func (channel *PlatformChannel) SetAvailableModelIDs(modelIDs []string) {
	if channel == nil {
		return
	}
	channel.AvailableModels = NormalizeChannelModelIDsPreserveOrder(modelIDs)
}

func (channel *PlatformChannel) SetChannelModels(configs []ChannelModel) {
	if channel == nil {
		return
	}
	normalized := NormalizeChannelModelsPreserveOrder(configs)
	for i := range normalized {
		completeChannelModelRowDefaults(&normalized[i], channel.GetChannelProtocol())
	}
	channel.ChannelModels = normalized

	available := make([]string, 0, len(normalized))
	selected := make([]string, 0, len(normalized))
	for _, row := range normalized {
		available = append(available, row.Model)
		if !row.Selected {
			continue
		}
		selected = append(selected, row.Model)
	}
	channel.SetAvailableModelIDs(available)
	channel.Models = JoinChannelModelCSV(selected)
}

func (channel *PlatformChannel) NormalizeChannelModelState() {
	if channel == nil {
		return
	}
	if channel.ChannelModelsProvided {
		channel.SetChannelModels(channel.ChannelModels)
		return
	}
	if len(channel.ChannelModels) > 0 {
		channel.SetChannelModels(channel.ChannelModels)
		return
	}
	if channel.ModelsProvided {
		channel.Models = JoinChannelModelCSV(ParseChannelModelCSV(channel.Models))
	}
}

func (channel *PlatformChannel) SetChannelTests(results []ChannelTest) {
	if channel == nil {
		return
	}
	channel.Tests = NormalizeChannelTestRows(results)
	channel.TestsLastTestedAt = CalcChannelTestsLastTestedAt(channel.Tests)
}

func (channel *PlatformChannel) Insert() error {
	return mustChannelRepo().Insert(channel)
}

func (channel *PlatformChannel) Update() error {
	return mustChannelRepo().Update(channel)
}

func (channel *PlatformChannel) UpdateResponseTime(responseTime int64) {
	mustChannelRepo().UpdateResponseTime(channel, responseTime)
}

func (channel *PlatformChannel) Delete() error {
	return mustChannelRepo().Delete(channel)
}

func (channel *PlatformChannel) LoadConfig() (ChannelConfig, error) {
	var cfg ChannelConfig
	if channel.Config == "" {
		return cfg, nil
	}
	err := json.Unmarshal([]byte(channel.Config), &cfg)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func UpdateChannelStatusById(id string, status int) error {
	return mustChannelRepo().UpdateChannelStatusById(id, status)
}

func UpdateChannelUsedQuota(id string, quota int64) {
	mustChannelRepo().UpdateChannelUsedQuota(id, quota)
}

func updateChannelUsedQuota(id string, quota int64) {
	mustChannelRepo().UpdateChannelUsedQuotaDirect(id, quota)
}

func DeleteChannelByStatus(status int64) (int64, error) {
	return mustChannelRepo().DeleteChannelByStatus(status)
}

func DeleteDisabledChannel() (int64, error) {
	return mustChannelRepo().DeleteDisabledChannel()
}

func (channel *PlatformChannel) selectedChannelModels() []ChannelModel {
	configs := channel.GetChannelModels()
	if len(configs) == 0 {
		return nil
	}
	selected := make([]ChannelModel, 0, len(configs))
	for _, row := range configs {
		if !IsChannelModelPublished(row) {
			continue
		}
		selected = append(selected, row)
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func (channel *PlatformChannel) GetSelectedChannelModels() []ChannelModel {
	if channel == nil {
		return nil
	}
	return channel.selectedChannelModels()
}
