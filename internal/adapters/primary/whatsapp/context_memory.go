package whatsapp

// DeleteContextMessage deletes a specific context message for a user in a conversation
func (a *WhatsAppAdapter) DeleteContextMessage(conversationID, userID string, index int) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Get the session key
	key := createSessionKey(userID, conversationID)
	
	// Check if we have context for this session
	a.memoryManager.mutex.Lock()
	defer a.memoryManager.mutex.Unlock()
	
	contextMessages, exists := a.memoryManager.context[key]
	if !exists || len(contextMessages) == 0 {
		return false
	}
	
	// Check if index is valid
	if index < 0 || index >= len(contextMessages) {
		return false
	}
	
	// Remove the context message at the specified index
	a.memoryManager.context[key] = append(contextMessages[:index], contextMessages[index+1:]...)
	
	return true
}
