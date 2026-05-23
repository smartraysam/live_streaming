# React Livestream Sample (Local)

This sample React app demonstrates how to use the local Go livestream service.

Workspace reference:

- Backend service: ../livestream-service
- Workspace root documentation: ../README.md

## What It Can Do

- List live streams
- Create a stream (requires JWT)
- Fetch playback URL (requires JWT + access checks)
- Purchase and verify tickets
- Send tip requests
- Connect to stream chat over WebSocket and send messages
- Manage private sessions in Sessions view:
   - Create session
   - Invite viewer to session
   - List incoming invites
   - Accept or decline invite

## Prerequisites

- Node.js 18+
- Running Go livestream service on http://localhost:8080
- Valid JWT from your Laravel auth flow for protected endpoints

## Setup

1. Install dependencies:

   npm install

2. Create env file:

   cp .env.example .env

3. Start the app:

   npm run dev

4. Open the URL shown by Vite (usually http://localhost:5173)

## Environment Variables

- VITE_API_BASE: defaults to /api/v1
- VITE_WS_BASE: defaults to ws://localhost:8080/api/v1

The Vite dev server includes a proxy for /api to http://localhost:8080, so you can use /api/v1 directly for HTTP calls.

## Local Workflow With Service

1. Start LocalStack + Go service (from the livestream-service project):

   cd ../livestream-service
   make local-up
   make local-init
   make local-run

2. Start this React sample in another terminal:

   cd ../react-livestream-sample
   npm install
   cp .env.example .env
   npm run dev

3. Paste a JWT token in the app and test the flows.

## UI Views

- Streams: stream CRUD/testing, playback/ticket/tip actions, and chat
- Sessions: private session lifecycle actions (create/invite/incoming/accept/decline)

## Notes

- Chat endpoint is protected; connect will fail without a valid token.
- If Laravel auth/payment mocks are not available locally, protected actions may return unauthorized/payment errors.
- You can still test public list endpoints without auth.
