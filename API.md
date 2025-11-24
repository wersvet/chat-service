# Chat Service API

## Authentication
All HTTP endpoints require the `Authorization: Bearer <JWT>` header. WebSocket connections accept either the same header or a `?token=` query parameter. Tokens are validated via the auth-service.

## REST Endpoints

### GET /chats
Returns visible chats for the authenticated user, including private and group entries.

**Response**
```
{
  "chats": [
    { "type": "private", "chat_id": 12, "friend_id": 42, "friend_username": "alice", "created_at": "2024-06-01T12:00:00Z" },
    { "type": "group", "group_id": 4, "name": "Team", "owner_id": 7, "created_at": "2024-06-01T12:00:00Z" }
  ]
}
```

### POST /chats/start
Starts (or retrieves) a chat with a friend.

**Body**
```
{ "friend_id": 42 }
```

**Response**
```
{ "chat_id": 12 }
```

### GET /chats/:chat_id/messages
Returns chat messages filtered by deletion flags.

**Response**
```
{
  "messages": [ { "id": 1, "content": "hi" } ]
}
```

### POST /chats/:chat_id/messages
Sends a message.

**Body**
```
{ "content": "hello" }
```

**Response**
```
{ "id": 1, "chat_id": 12, "content": "hello", ... }
```

### DELETE /chats/:chat_id/messages/:message_id/me
Marks a message as deleted for the caller only.

### DELETE /chats/:chat_id/messages/:message_id/all
Marks a message as deleted for both members (sender only). Broadcasts a WebSocket `delete_for_all` event.

### DELETE /chats/:chat_id/me
Hides the chat for the caller via `chat_visibility`.

### POST /groups
Creates a group. The caller becomes the owner and is added automatically.

**Body**
```
{ "name": "Our Group", "member_ids": [2,3] }
```

**Response**
```
{ "group_id": 5 }
```

### GET /groups
Lists groups the caller is a member of.

**Response**
```
{ "groups": [ { "id": 5, "name": "Our Group", "owner_id": 1 } ] }
```

### GET /groups/:group_id/messages
Returns messages for a group the caller belongs to.

### POST /groups/:group_id/messages
Sends a message to the group.

**Body**
```
{ "content": "hello group" }
```

### DELETE /groups/:group_id/messages/:message_id/all
Marks a group message as deleted for everyone (sender only) and broadcasts a `delete_for_all` event.

## WebSocket

### GET /ws/chats/:chat_id
- Requires a valid JWT (header or `token` query param).
- Confirms the caller is part of the chat.
- Broadcasts:
  - `{"type":"message","message":{...}}` for new messages.
  - `{"type":"delete_for_all","message_id":123}` when a message is deleted for everyone.

Clients should keep the socket open and handle these events to stay synchronized.

### GET /ws/groups/:group_id
- Requires a valid JWT (header or `token` query param).
- Confirms the caller is a member of the group.
- Broadcasts the same event shapes as private chats for new messages and delete-for-all operations.

## Environment

The service communicates with companion services over gRPC using these variables (defaults in parentheses):

- `AUTH_GRPC_ADDR` (`localhost:8084`) — auth-service gRPC address used for token validation.
- `USER_GRPC_ADDR` (`localhost:8085`) — user-service gRPC address used for friendship and user lookups.
