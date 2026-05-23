# Live Streaming Workspace

This repository contains a Go livestream backend service and a React sample client for local development.

## Projects

- livestream-service: Go microservice for stream lifecycle, private sessions, chat, ticket access, and Laravel-coordinated payments.
- react-livestream-sample: React (Vite) app to test the service locally through REST and WebSocket flows.

## Folder Structure

- livestream-service-prompt.md: original implementation prompt/spec.
- livestream-service/: backend service code.
- react-livestream-sample/: frontend sample app.

## Quick Start (Local)

1. Start backend local dependencies and service:

   cd livestream-service
   make local-up
   make local-init
   make local-run

2. Start React sample app in a second terminal:

   cd react-livestream-sample
   npm install
   cp .env.example .env
   npm run dev

3. Open the Vite URL (usually http://localhost:5173).

## Backend Docs

See livestream-service/README.md for full backend details, environment variables, API endpoints, and Swagger usage.

## Frontend Docs

See react-livestream-sample/README.md for full sample-app usage and private session flows.

## Notes

- The Go service delegates auth and payments to Laravel internal APIs.
- In local mode, IVS can be mocked while DynamoDB and SQS run via LocalStack.
