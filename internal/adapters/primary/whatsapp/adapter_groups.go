package whatsapp

import (
	"fmt"
	"strings"

	"github.com/vibin/chat-bot/internal/core/ports"
	"go.mau.fi/whatsmeow/types"
)

// GetGroups returns a list of all available WhatsApp groups
func (a *WhatsAppAdapter) GetGroups() ([]ports.GroupInfo, error) {
	if a.client == nil || !a.client.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client not connected")
	}

	// Get all groups
	groups, err := a.client.GetJoinedGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to get groups: %v", err)
	}

	// Convert to GroupInfo and check if allowed
	var groupInfos []ports.GroupInfo
	for _, group := range groups {
		groupID := group.JID.String()
		isAllowed := a.isGroupAllowed(groupID)

		info := ports.GroupInfo{
			ID:          groupID,
			Name:        a.getGroupNameFromInfo(*group),
			MemberCount: len(group.Participants),
			IsAllowed:   isAllowed,
		}
		groupInfos = append(groupInfos, info)
	}

	return groupInfos, nil
}

// UpdateAllowedGroups updates the list of allowed groups
func (a *WhatsAppAdapter) UpdateAllowedGroups(groups []string) error {
	// Update config
	a.mutex.Lock()
	a.config.AllowedGroups = groups
	a.mutex.Unlock()
	
	return nil
}

// getGroupNameFromInfo gets a readable group name from group info
func (a *WhatsAppAdapter) getGroupNameFromInfo(group types.GroupInfo) string {
	if group.Name != "" {
		return group.Name
	}
	
	// Try to extract a name from the JID if no name available
	parts := strings.Split(group.JID.String(), "@")
	if len(parts) > 0 {
		return parts[0]
	}
	
	return group.JID.String()
}
