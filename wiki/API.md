# API Reference

The Monitoring Assistant exposes a set of RESTful API endpoints for interacting with the application. All endpoints are prefixed with `/api`.

## Authentication

All API endpoints require authentication. The assistant uses the Grafana session cookie to authenticate and identify the user.

## Endpoints

### Chat

#### `POST /api/chat`

This is the main endpoint for interacting with the chat assistant. It uses Server-Sent Events (SSE) to stream the response back to the client.

**Request Body:**

*   `message` (string, required): The user's message.
*   `session_id` (string, optional): The ID of the chat session to continue. If not provided, a new session will be created.
*   `dashboard_context` (object, optional): Information about the current Grafana dashboard context.

**Response:**

A stream of Server-Sent Events. The `type` field of the event indicates the type of message. Common types include:
*   `message`: A chunk of the assistant's response.
*   `tool_call`: The assistant is about to call a tool.
*   `tool_result`: The result of a tool call.
*   `session_id`: The ID of the created chat session.

### History

#### `GET /api/history`

Retrieves a list of all chat sessions for the current user.

**Query Parameters:**

*   `dashboard_uid` (string, optional): Filter sessions by dashboard UID.

**Response:**

An array of `HistorySession` objects.

#### `GET /api/history/{id}`

Retrieves a specific chat session and its messages.

**Response:**

A `HistoryDetail` object, which includes the session information and an array of `HistoryMessage` objects.

#### `DELETE /api/history/{id}`

Deletes a specific chat session.

### Feedback

#### `POST /api/feedback`

Submits feedback for a specific assistant message.

**Request Body:**

*   `session_id` (string, required): The ID of the chat session.
*   `message_id` (string, required): The ID of the message being rated.
*   `rating` (integer, required): A rating from 1 to 5.
*   `comment` (string, optional): A text comment.

### User

#### `GET /api/user`

Returns information about the currently authenticated Grafana user.

**Response:**

A `CurrentUser` object.

## Data Structures

Key data structures used in the API.

### `ChatRequest`

```json
{
  "message": "string",
  "session_id": "string",
  "dashboard_context": {
    "uid": "string",
    "name": "string",
    "folder": "string",
    "tags": ["string"],
    "time_range": {"from": "string", "to": "string"},
    "variables": {"var_name": "value"},
    "explore": {
      "datasource": "string",
      "queries": ["string"]
    }
  }
}
```

### `HistorySession`

```json
{
  "id": "string",
  "title": "string",
  "dashboard_uid": "string",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

### `CurrentUser`

```json
{
  "id": "integer",
  "login": "string",
  "name": "string",
  "email": "string",
  "org_id": "integer"
}
```
