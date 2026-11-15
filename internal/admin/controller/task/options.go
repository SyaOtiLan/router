package task

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
)

const taskFilterOptionLimit = 200

type taskFilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type taskFilterOptions struct {
	Models   []taskFilterOption `json:"models"`
	Channels []taskFilterOption `json:"channels,omitempty"`
	Users    []taskFilterOption `json:"users,omitempty"`
}

func distinctTaskValues(tableName string, queryColumn string, userID string) ([]string, error) {
	rows := make([]string, 0)
	query := model.DB.Table(tableName).
		Distinct(queryColumn).
		Where("COALESCE(" + queryColumn + ", '') <> ''")
	if strings.TrimSpace(userID) != "" {
		query = query.Where("user_id = ?", strings.TrimSpace(userID))
	}
	if err := query.Order(queryColumn+" asc").Limit(taskFilterOptionLimit).Pluck(queryColumn, &rows).Error; err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		value := strings.TrimSpace(row)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result, nil
}

func buildPlainTaskOptions(values []string) []taskFilterOption {
	options := make([]taskFilterOption, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		options = append(options, taskFilterOption{
			Value: normalized,
			Label: normalized,
		})
	}
	return options
}

func loadTaskChannelOptions(channelIDs []string) ([]taskFilterOption, error) {
	normalizedIDs := make([]string, 0, len(channelIDs))
	seen := make(map[string]struct{}, len(channelIDs))
	for _, id := range channelIDs {
		value := strings.TrimSpace(id)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalizedIDs = append(normalizedIDs, value)
	}
	if len(normalizedIDs) == 0 {
		return []taskFilterOption{}, nil
	}
	var channels []*model.PlatformChannel
	if err := model.DB.Select("id", "name").Where("id IN ?", normalizedIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	nameByID := make(map[string]string, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		nameByID[strings.TrimSpace(channel.Id)] = strings.TrimSpace(channel.DisplayName())
	}
	options := make([]taskFilterOption, 0, len(normalizedIDs))
	for _, id := range normalizedIDs {
		label := strings.TrimSpace(nameByID[id])
		if label == "" {
			label = id
		}
		options = append(options, taskFilterOption{
			Value: id,
			Label: label,
		})
	}
	return options, nil
}

func loadTaskUserOptions(userIDs []string) ([]taskFilterOption, error) {
	normalizedIDs := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		value := strings.TrimSpace(id)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalizedIDs = append(normalizedIDs, value)
	}
	if len(normalizedIDs) == 0 {
		return []taskFilterOption{}, nil
	}
	var users []*model.User
	if err := model.DB.Select("id", "username").Where("id IN ?", normalizedIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	nameByID := make(map[string]string, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		nameByID[strings.TrimSpace(user.Id)] = strings.TrimSpace(user.Username)
	}
	options := make([]taskFilterOption, 0, len(normalizedIDs))
	for _, id := range normalizedIDs {
		label := strings.TrimSpace(nameByID[id])
		if label == "" {
			label = id
		}
		options = append(options, taskFilterOption{
			Value: id,
			Label: label,
		})
	}
	return options, nil
}

func buildAdminTaskFilterOptions() (taskFilterOptions, error) {
	models, err := distinctTaskValues(model.AdminTasksTableName, "model", "")
	if err != nil {
		return taskFilterOptions{}, err
	}
	channelIDs, err := distinctTaskValues(model.AdminTasksTableName, "channel_id", "")
	if err != nil {
		return taskFilterOptions{}, err
	}
	channels, err := loadTaskChannelOptions(channelIDs)
	if err != nil {
		return taskFilterOptions{}, err
	}
	return taskFilterOptions{
		Models:   buildPlainTaskOptions(models),
		Channels: channels,
	}, nil
}

func buildUserTaskFilterOptions(userID string, includeUsers bool) (taskFilterOptions, error) {
	models, err := distinctTaskValues(model.UserTasksTableName, "model", userID)
	if err != nil {
		return taskFilterOptions{}, err
	}
	channelIDs, err := distinctTaskValues(model.UserTasksTableName, "channel_id", userID)
	if err != nil {
		return taskFilterOptions{}, err
	}
	channels, err := loadTaskChannelOptions(channelIDs)
	if err != nil {
		return taskFilterOptions{}, err
	}
	options := taskFilterOptions{
		Models:   buildPlainTaskOptions(models),
		Channels: channels,
	}
	if !includeUsers {
		return options, nil
	}
	userIDs, err := distinctTaskValues(model.UserTasksTableName, "user_id", "")
	if err != nil {
		return taskFilterOptions{}, err
	}
	users, err := loadTaskUserOptions(userIDs)
	if err != nil {
		return taskFilterOptions{}, err
	}
	options.Users = users
	return options, nil
}

func GetTaskFilterOptions(c *gin.Context) {
	options, err := buildAdminTaskFilterOptions()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

func GetAdminUserTaskFilterOptions(c *gin.Context) {
	options, err := buildUserTaskFilterOptions("", true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

func GetCurrentUserTaskFilterOptions(c *gin.Context) {
	options, err := buildUserTaskFilterOptions(c.GetString(ctxkey.Id), false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}
