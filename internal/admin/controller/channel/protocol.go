package channel

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/internal/admin/model"
)

type channelProtocolOptionItem struct {
	Key         string `json:"key"`
	Text        string `json:"text"`
	Value       string `json:"value"`
	Name        string `json:"name,omitempty"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
	Tip         string `json:"tip,omitempty"`
}

func GetChannelProtocols(c *gin.Context) {
	rows := make([]model.ChannelProtocolCatalog, 0)
	if err := model.DB.
		Where("enabled = ?", true).
		Order("sort_order asc, id asc").
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "读取协议类型配置失败: " + err.Error(),
		})
		return
	}

	options := make([]channelProtocolOptionItem, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			continue
		}
		label := strings.TrimSpace(row.Label)
		if label == "" {
			label = name
		}
		if label == "" {
			label = "unknown"
		}
		options = append(options, channelProtocolOptionItem{
			Key:         name,
			Value:       name,
			Text:        label,
			Name:        name,
			Color:       strings.TrimSpace(row.Color),
			Description: strings.TrimSpace(row.Description),
			Tip:         strings.TrimSpace(row.Tip),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}
