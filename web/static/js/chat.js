// chat.js - Handles the chat page functionality

document.addEventListener('DOMContentLoaded', () => {
    // Get DOM elements
    const messagesContainer = document.getElementById('messages');
    const messageForm = document.getElementById('message-form');
    const messageInput = document.getElementById('message-input');
    const chatTitle = document.getElementById('chat-title');
    
    // Get chat ID from URL
    const chatId = window.location.pathname.split('/').pop();
    
    // Fetch and display chat
    fetchChat();
    
    // Add event listener for form submission
    messageForm.addEventListener('submit', sendMessage);
    
    /**
     * Fetches the chat from the API and displays the messages
     */
    async function fetchChat() {
        try {
            const response = await fetch(`/api/chats/${chatId}`);
            if (!response.ok) {
                throw new Error('Chat not found');
            }
            
            const chat = await response.json();
            
            // Update chat title
            chatTitle.textContent = chat.title || 'Untitled Chat';
            
            // Display messages
            displayMessages(chat.messages);
        } catch (error) {
            console.error('Error fetching chat:', error);
            messagesContainer.innerHTML = `
                <div class="empty-state">
                    <p>Failed to load chat. Please try again later.</p>
                </div>
            `;
        }
    }
    
    /**
     * Displays the chat messages
     */
    function displayMessages(messages) {
        if (messages.length === 0) {
            messagesContainer.innerHTML = `
                <div class="empty-state">
                    <p>Start chatting with the LLM by typing a message below!</p>
                </div>
            `;
            return;
        }
        
        // Clear the messages container
        messagesContainer.innerHTML = '';
        
        // Add each message to the container
        messages.forEach(message => {
            const messageElement = document.createElement('div');
            messageElement.className = `message ${message.role}-message`;
            
            // Format the message content with proper line breaks
            const formattedContent = message.content
                .replace(/\n/g, '<br>')
                // Add syntax highlighting for code blocks (simplified version)
                .replace(/```(\w*)([\s\S]*?)```/g, '<pre><code>$2</code></pre>');
            
            messageElement.innerHTML = formattedContent;
            messagesContainer.appendChild(messageElement);
        });
        
        // Scroll to the bottom
        scrollToBottom();
    }
    
    /**
     * Sends a message to the API and updates the UI
     */
    async function sendMessage(event) {
        event.preventDefault();
        
        const content = messageInput.value.trim();
        if (!content) return;
        
        // Clear input
        messageInput.value = '';
        
        try {
            // Add user message to UI immediately
            addMessage('user', content);
            
            // Add loading indicator
            const loadingElement = document.createElement('div');
            loadingElement.className = 'loader';
            messagesContainer.appendChild(loadingElement);
            scrollToBottom();
            
            // Send message to API
            const response = await fetch(`/api/chats/${chatId}/messages`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ content }),
            });
            
            if (!response.ok) {
                throw new Error('Failed to send message');
            }
            
            const chat = await response.json();
            
            // Remove loading indicator
            messagesContainer.removeChild(loadingElement);
            
            // Display all messages to ensure we have the latest state
            displayMessages(chat.messages);
        } catch (error) {
            console.error('Error sending message:', error);
            
            // Remove loading indicator
            const loader = messagesContainer.querySelector('.loader');
            if (loader) {
                messagesContainer.removeChild(loader);
            }
            
            // Add error message
            addMessage('assistant', 'Sorry, there was an error processing your message. Please try again.');
        }
    }
    
    /**
     * Adds a message to the UI
     */
    function addMessage(role, content) {
        const messageElement = document.createElement('div');
        messageElement.className = `message ${role}-message`;
        messageElement.textContent = content;
        messagesContainer.appendChild(messageElement);
        scrollToBottom();
    }
    
    /**
     * Scrolls the messages container to the bottom
     */
    function scrollToBottom() {
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
});
