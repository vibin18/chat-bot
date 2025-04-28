// main.js - Handles the home page functionality

document.addEventListener('DOMContentLoaded', () => {
    // Get DOM elements
    const chatsList = document.getElementById('chats-list');
    const newChatBtn = document.getElementById('new-chat-btn');
    const modelDetails = document.getElementById('model-details');
    
    // Fetch and display chats
    fetchChats();
    
    // Fetch and display model info
    fetchModelInfo();
    
    // Add event listener for new chat button
    newChatBtn.addEventListener('click', createNewChat);
    
    /**
     * Fetches the list of chats from the API and displays them
     */
    async function fetchChats() {
        try {
            const response = await fetch('/api/chats');
            const chats = await response.json();
            
            if (chats.length === 0) {
                chatsList.innerHTML = `
                    <div class="empty-state">
                        <p>No conversations yet. Start a new chat!</p>
                    </div>
                `;
                return;
            }
            
            // Clear the chats list
            chatsList.innerHTML = '';
            
            // Sort chats by updated_at in descending order
            chats.sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));
            
            // Add each chat to the list
            chats.forEach(chat => {
                const lastMessage = chat.messages.length > 0 
                    ? chat.messages[chat.messages.length - 1].content.substring(0, 100) + (chat.messages[chat.messages.length - 1].content.length > 100 ? '...' : '')
                    : 'No messages yet';
                
                const chatElement = document.createElement('div');
                chatElement.className = 'chat-item';
                chatElement.innerHTML = `
                    <h3>${chat.title || 'Untitled Chat'}</h3>
                    <p>${lastMessage}</p>
                    <p><small>${formatDate(chat.updated_at)}</small></p>
                `;
                
                // Add click event to navigate to chat page
                chatElement.addEventListener('click', () => {
                    window.location.href = `/chat/${chat.id}`;
                });
                
                chatsList.appendChild(chatElement);
            });
        } catch (error) {
            console.error('Error fetching chats:', error);
            chatsList.innerHTML = `
                <div class="empty-state">
                    <p>Failed to load chats. Please try again later.</p>
                </div>
            `;
        }
    }
    
    /**
     * Creates a new chat
     */
    async function createNewChat() {
        try {
            const title = `Chat ${new Date().toLocaleString()}`;
            
            const response = await fetch('/api/chats', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ title }),
            });
            
            const newChat = await response.json();
            
            // Navigate to the new chat
            window.location.href = `/chat/${newChat.id}`;
        } catch (error) {
            console.error('Error creating new chat:', error);
            alert('Failed to create a new chat. Please try again later.');
        }
    }
    
    /**
     * Fetches model information from the API and displays it
     */
    async function fetchModelInfo() {
        try {
            const response = await fetch('/api/model');
            const modelInfo = await response.json();
            
            let html = '';
            for (const [key, value] of Object.entries(modelInfo)) {
                html += `<div><strong>${formatKey(key)}:</strong></div><div>${value}</div>`;
            }
            
            modelDetails.innerHTML = html;
        } catch (error) {
            console.error('Error fetching model info:', error);
            modelDetails.innerHTML = '<p>Failed to load model information. Please try again later.</p>';
        }
    }
    
    /**
     * Formats a date string to a human-readable format
     */
    function formatDate(dateString) {
        const date = new Date(dateString);
        return date.toLocaleString();
    }
    
    /**
     * Formats a key string to a more readable format
     */
    function formatKey(key) {
        // Convert camelCase to Title Case with spaces
        return key
            .replace(/([A-Z])/g, ' $1')
            .replace(/^./, str => str.toUpperCase());
    }
});
