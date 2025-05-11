#!/bin/bash

# This script removes duplicate JavaScript functions from memory_admin.html
# Create a backup
cp /Users/vibin/chat-bot/web/templates/memory_admin.html /Users/vibin/chat-bot/web/templates/memory_admin.html.bak.$(date +%s)

# Extract the file until the second script tag (before duplicates start)
sed -n '1,/<script>/p' /Users/vibin/chat-bot/web/templates/memory_admin.html | head -n -1 > /Users/vibin/chat-bot/web/templates/memory_admin.html.fixed

# Add the first set of JavaScript code (keeping the debug-enhanced version)
sed -n '/<script>/,/function clearAllMemories/p' /Users/vibin/chat-bot/web/templates/memory_admin.html >> /Users/vibin/chat-bot/web/templates/memory_admin.html.fixed

# Add the closing script and html tags
echo "    </script>" >> /Users/vibin/chat-bot/web/templates/memory_admin.html.fixed
echo "</body>" >> /Users/vibin/chat-bot/web/templates/memory_admin.html.fixed
echo "</html>" >> /Users/vibin/chat-bot/web/templates/memory_admin.html.fixed

# Replace the original with the fixed version
mv /Users/vibin/chat-bot/web/templates/memory_admin.html.fixed /Users/vibin/chat-bot/web/templates/memory_admin.html

echo "Fixed memory_admin.html - duplicate JavaScript functions removed"
