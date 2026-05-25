#!/usr/bin/env node
/**
 * Mock Laravel internal API server for local Go service development.
 * Listens on port 18000 (or PORT env var).
 *
 * Supported endpoints:
 *   POST /internal/auth/verify
 *   POST /internal/payments/charge
 *   POST /internal/notifications/send
 *   POST /internal/streams/access-check   ← follow/subscribe check (free streams)
 *   POST /internal/streams/sync           ← persist stream metadata in Laravel
 */

const http = require('http');

const PORT = process.env.PORT || 18000;

// Known tokens → user info
const TOKENS = {
  'creator-token': { user_id: 'creator-1', role: 'creator', username: 'creator_demo' },
  'viewer-token':  { user_id: 'viewer-1',  role: 'viewer',  username: 'viewer_demo'  },
};

// Users that follow creator-1 (viewer-1 does, so free streams are accessible)
const FOLLOWS = new Set(['viewer-1:creator-1']);

// In-memory store for synced streams
const syncedStreams = {};

function readBody(req) {
  return new Promise((resolve, reject) => {
    let data = '';
    req.on('data', chunk => { data += chunk; });
    req.on('end', () => {
      try { resolve(JSON.parse(data || '{}')); }
      catch { resolve({}); }
    });
    req.on('error', reject);
  });
}

function respond(res, status, body) {
  const json = JSON.stringify(body);
  res.writeHead(status, { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(json) });
  res.end(json);
}

const server = http.createServer(async (req, res) => {
  const body = await readBody(req);
  console.log(`[mock-laravel] ${req.method} ${req.url}`, body);

  // ── Auth verify ──────────────────────────────────────────────────────────
  if (req.method === 'POST' && req.url === '/internal/auth/verify') {
    const user = TOKENS[body.token];
    if (!user) return respond(res, 401, { error: 'invalid_token' });
    return respond(res, 200, user);
  }

  // ── Payment charge ────────────────────────────────────────────────────────
  if (req.method === 'POST' && req.url === '/internal/payments/charge') {
    return respond(res, 200, {
      transaction_id: `txn-demo-${Date.now()}`,
      status: 'success',
    });
  }

  // ── Notification send ─────────────────────────────────────────────────────
  if (req.method === 'POST' && req.url === '/internal/notifications/send') {
    return respond(res, 200, { sent: true });
  }

  // ── Stream access check (for free / unlocked streams) ─────────────────────
  // Body: { user_id, creator_id }
  // Logic: allow if the user follows OR subscribes to the creator.
  if (req.method === 'POST' && req.url === '/internal/streams/access-check') {
    const { user_id, creator_id } = body;
    if (!user_id || !creator_id) return respond(res, 400, { error: 'missing_fields' });

    // Creator always has access to their own content
    if (user_id === creator_id) {
      return respond(res, 200, { can_access: true, reason: 'creator' });
    }

    const followKey = `${user_id}:${creator_id}`;
    if (FOLLOWS.has(followKey)) {
      return respond(res, 200, { can_access: true, reason: 'following' });
    }

    return respond(res, 200, { can_access: false, reason: 'not_following' });
  }

  // ── Stream sync ───────────────────────────────────────────────────────────
  // Body: stream fields (see pkg/laravel SyncStreamRequest)
  if (req.method === 'POST' && req.url === '/internal/streams/sync') {
    const { stream_id } = body;
    if (!stream_id) return respond(res, 400, { error: 'missing_stream_id' });

    // Upsert into our in-memory store and give back a deterministic Laravel ID
    if (!syncedStreams[stream_id]) {
      syncedStreams[stream_id] = { laravel_stream_id: `lv-${Object.keys(syncedStreams).length + 1}` };
    }
    syncedStreams[stream_id] = { ...syncedStreams[stream_id], ...body, updated_at: new Date().toISOString() };

    console.log(`[mock-laravel] synced streams:`, Object.keys(syncedStreams));
    return respond(res, 200, {
      synced: true,
      laravel_stream_id: syncedStreams[stream_id].laravel_stream_id,
    });
  }

  respond(res, 404, { error: 'not_found' });
});

server.listen(PORT, () => {
  console.log(`[mock-laravel] listening on http://localhost:${PORT}`);
  console.log('[mock-laravel] tokens: creator-token → creator-1 | viewer-token → viewer-1');
  console.log('[mock-laravel] endpoints: /internal/auth/verify, /internal/payments/charge,');
  console.log('              /internal/notifications/send, /internal/streams/access-check,');
  console.log('              /internal/streams/sync');
});
