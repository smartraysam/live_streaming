# React Livestream Sample (Local)

This sample React app demonstrates how to use the local Go livestream service.

Workspace reference:

- Backend service: ../livestream-service
- Workspace root documentation: ../README.md

## What It Can Do

- Create a broadcast channel as a creator
- Load creator broadcasts and select one stream
- Fetch ingest credentials (ingest endpoint + stream key) for OBS
- Run watch flow as one viewer (1-to-1 watch test)
- Run watch flow as many viewers (1-to-many watch test)
- Auto-handle ticket purchase and verification for paid streams
- Preview playback with Video.js

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

- Connection + creator identity setup
- Broadcast channel creation and creator stream selection
- OBS ingest credentials display for broadcast start
- One-viewer and many-viewer watch flow controls
- Live playback preview with Video.js

## Notes

- Chat endpoint is protected; connect will fail without a valid token.
- If Laravel auth/payment mocks are not available locally, protected actions may return unauthorized/payment errors.
- You can still test public list endpoints without auth.
