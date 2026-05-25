import { useMemo, useRef, useState } from 'react';

const API_BASE = import.meta.env.VITE_API_BASE || '/api/v1';
const WS_BASE = import.meta.env.VITE_WS_BASE || 'ws://localhost:8080/api/v1';

function parseJsonSafe(text) {
  try {
    return JSON.parse(text);
  } catch {
    return { raw: text };
  }
}

export default function App() {
  const [token, setToken] = useState('');
  const [view, setView] = useState('streams');
  const [streams, setStreams] = useState([]);
  const [selectedStreamId, setSelectedStreamId] = useState('');
  const [playbackUrl, setPlaybackUrl] = useState('');
  const [status, setStatus] = useState('Ready.');

  const [createForm, setCreateForm] = useState({
    stream_type: 'broadcast',
    title: 'Local Sample Stream',
    description: 'Created from React local sample',
    is_paid: false,
    ticket_price_usd: 0,
    invited_viewer_id: ''
  });

  const [tipAmount, setTipAmount] = useState('5');
  const [tipMessage, setTipMessage] = useState('great stream!');

  const [createSessionForm, setCreateSessionForm] = useState({
    title: '1:1 Coaching Session',
    description: 'Private session from React sample',
    invited_viewer_id: '',
    price_usd: 15
  });
  const [sessionActionId, setSessionActionId] = useState('');
  const [sessionInviteViewerId, setSessionInviteViewerId] = useState('');
  const [incomingSessions, setIncomingSessions] = useState([]);

  const [chatMessages, setChatMessages] = useState([]);
  const [chatInput, setChatInput] = useState('');
  const [accessResult, setAccessResult] = useState(null);
  const [syncResult, setSyncResult] = useState(null);
  const wsRef = useRef(null);

  const authHeaders = useMemo(() => {
    if (!token.trim()) {
      return {};
    }
    return { Authorization: `Bearer ${token.trim()}` };
  }, [token]);

  async function request(path, options = {}, requireAuth = false) {
    const headers = {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
      ...(requireAuth ? authHeaders : {})
    };

    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers
    });

    const text = await res.text();
    const json = parseJsonSafe(text);

    if (!res.ok) {
      const msg = json?.error || `HTTP ${res.status}`;
      throw new Error(msg);
    }

    return json;
  }

  async function listStreams() {
    try {
      setStatus('Loading streams...');
      const json = await request('/streams');
      setStreams(Array.isArray(json.data) ? json.data : []);
      setStatus('Loaded streams.');
    } catch (err) {
      setStatus(`List streams failed: ${err.message}`);
    }
  }

  async function createStream() {
    try {
      setStatus('Creating stream...');
      const payload = {
        ...createForm,
        ticket_price_usd: Number(createForm.ticket_price_usd || 0)
      };
      const json = await request('/streams', {
        method: 'POST',
        body: JSON.stringify(payload)
      }, true);
      setStatus(`Created stream: ${json?.data?.stream_id || 'ok'}`);
      await listStreams();
    } catch (err) {
      setStatus(`Create stream failed: ${err.message}`);
    }
  }

  async function getPlayback() {
    if (!selectedStreamId.trim()) {
      setStatus('Select or enter a stream id first.');
      return;
    }
    try {
      setStatus('Fetching playback URL...');
      const json = await request(`/streams/${selectedStreamId}/playback`, {}, true);
      setPlaybackUrl(json?.data?.playback_url || '');
      setStatus('Playback URL fetched.');
    } catch (err) {
      setStatus(`Playback request failed: ${err.message}`);
    }
  }

  async function sendTip() {
    if (!selectedStreamId.trim()) {
      setStatus('Select or enter a stream id first.');
      return;
    }
    try {
      setStatus('Sending tip...');
      await request(`/streams/${selectedStreamId}/tip`, {
        method: 'POST',
        body: JSON.stringify({ amount_usd: Number(tipAmount), message: tipMessage })
      }, true);
      setStatus('Tip request submitted.');
    } catch (err) {
      setStatus(`Tip failed: ${err.message}`);
    }
  }

  async function purchaseTicket() {
    if (!selectedStreamId.trim()) {
      setStatus('Select or enter a stream id first.');
      return;
    }
    try {
      setStatus('Purchasing ticket...');
      await request(`/streams/${selectedStreamId}/ticket/purchase`, { method: 'POST' }, true);
      setStatus('Ticket purchase request succeeded.');
    } catch (err) {
      setStatus(`Ticket purchase failed: ${err.message}`);
    }
  }

  async function verifyTicket() {
    if (!selectedStreamId.trim()) {
      setStatus('Select or enter a stream id first.');
      return;
    }
    try {
      setStatus('Verifying ticket...');
      const json = await request(`/streams/${selectedStreamId}/ticket/verify`, {}, true);
      setStatus(`Ticket verify result: ${json?.data?.has_ticket ? 'valid' : 'not found'}`);
    } catch (err) {
      setStatus(`Ticket verify failed: ${err.message}`);
    }
  }

  async function createSession() {
    try {
      setStatus('Creating private session...');
      const payload = {
        ...createSessionForm,
        price_usd: Number(createSessionForm.price_usd || 0)
      };
      const json = await request('/sessions', {
        method: 'POST',
        body: JSON.stringify(payload)
      }, true);
      const id = json?.data?.stream_id || json?.data?.id || 'ok';
      setSessionActionId(id === 'ok' ? sessionActionId : id);
      setStatus(`Session created: ${id}`);
      await listStreams();
    } catch (err) {
      setStatus(`Create session failed: ${err.message}`);
    }
  }

  async function inviteToSession() {
    if (!sessionActionId.trim()) {
      setStatus('Enter a session id first.');
      return;
    }
    if (!sessionInviteViewerId.trim()) {
      setStatus('Enter viewer id to invite.');
      return;
    }
    try {
      setStatus('Sending session invite...');
      await request(`/sessions/${sessionActionId}/invite`, {
        method: 'POST',
        body: JSON.stringify({ viewer_id: sessionInviteViewerId })
      }, true);
      setStatus('Invite sent.');
    } catch (err) {
      setStatus(`Invite failed: ${err.message}`);
    }
  }

  async function loadIncomingSessions() {
    try {
      setStatus('Loading incoming sessions...');
      const json = await request('/sessions/incoming', {}, true);
      setIncomingSessions(Array.isArray(json?.data) ? json.data : []);
      setStatus('Incoming sessions loaded.');
    } catch (err) {
      setStatus(`Load incoming sessions failed: ${err.message}`);
    }
  }

  async function acceptSession(sessionId) {
    const id = (sessionId || sessionActionId).trim();
    if (!id) {
      setStatus('Enter a session id first.');
      return;
    }
    try {
      setStatus('Accepting session...');
      await request(`/sessions/${id}/accept`, { method: 'POST' }, true);
      setStatus('Session accepted.');
      await loadIncomingSessions();
    } catch (err) {
      setStatus(`Accept session failed: ${err.message}`);
    }
  }

  async function declineSession(sessionId) {
    const id = (sessionId || sessionActionId).trim();
    if (!id) {
      setStatus('Enter a session id first.');
      return;
    }
    try {
      setStatus('Declining session...');
      await request(`/sessions/${id}/decline`, { method: 'POST' }, true);
      setStatus('Session declined.');
      await loadIncomingSessions();
    } catch (err) {
      setStatus(`Decline session failed: ${err.message}`);
    }
  }

  async function checkAccess() {
    if (!selectedStreamId.trim()) {
      setStatus('Select or enter a stream id first.');
      return;
    }
    try {
      setStatus('Checking stream access...');
      const json = await request(`/streams/${selectedStreamId}/access`, {}, true);
      setAccessResult(json?.data || json);
      const d = json?.data || json;
      setStatus(d?.can_access
        ? `Access granted (${d.reason})`
        : `Access denied (${d.reason})`);
    } catch (err) {
      setAccessResult({ can_access: false, reason: err.message });
      setStatus(`Access check failed: ${err.message}`);
    }
  }

  async function syncToLaravel() {
    if (!selectedStreamId.trim()) {
      setStatus('Select or enter a stream id first.');
      return;
    }
    try {
      setStatus('Syncing stream to Laravel...');
      const json = await request(`/streams/${selectedStreamId}/sync`, { method: 'POST' }, true);
      setSyncResult(json?.data || json);
      setStatus(`Stream synced. Laravel ID: ${json?.data?.laravel_stream_id || 'ok'}`);
    } catch (err) {
      setSyncResult({ synced: false, error: err.message });
      setStatus(`Sync failed: ${err.message}`);
    }
  }

  function connectChat() {
    if (!selectedStreamId.trim()) {
      setStatus('Select or enter a stream id first.');
      return;
    }
    if (!token.trim()) {
      setStatus('A JWT token is required for chat connection.');
      return;
    }

    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    // Browser WebSocket does not allow custom headers, so the bearer token is
    // appended as a query param. The Go middleware checks ?token= as a fallback.
    const wsURL = `${WS_BASE}/streams/${selectedStreamId}/chat?token=${encodeURIComponent(token.trim())}`;
    const ws = new WebSocket(wsURL);
    wsRef.current = ws;

    ws.onopen = () => setStatus('Chat connected.');
    ws.onmessage = (event) => {
      const payload = parseJsonSafe(event.data);
      if (Array.isArray(payload)) {
        setChatMessages(payload);
        return;
      }
      setChatMessages((prev) => [...prev, payload]);
    };
    ws.onerror = () => setStatus('Chat socket error.');
    ws.onclose = () => setStatus('Chat disconnected.');
  }

  function disconnectChat() {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
      setStatus('Chat disconnected.');
    }
  }

  function sendChatMessage() {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      setStatus('Chat is not connected.');
      return;
    }
    const body = chatInput.trim();
    if (!body) {
      return;
    }
    wsRef.current.send(JSON.stringify({ type: 'message', body }));
    setChatInput('');
  }

  return (
    <div className="page">
      <header className="hero">
        <h1>Livestream Local React Sample</h1>
        <p>Test the Go livestream service on localhost with REST + WebSocket.</p>
      </header>

      <section className="panel">
        <h2>Connection</h2>
        <label>JWT token (from Laravel auth flow)</label>
        <input
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="Paste bearer token"
        />
        <small>Base API: {API_BASE} | WS: {WS_BASE}</small>
      </section>

      <section className="panel">
        <h2>Views</h2>
        <div className="actions">
          <button onClick={() => setView('streams')}>Streams</button>
          <button onClick={() => setView('sessions')}>Sessions</button>
        </div>
      </section>

      {view === 'streams' && <section className="grid">
        <div className="panel">
          <h2>Streams</h2>
          <button onClick={listStreams}>List All Streams</button>
          <div className="list">
            {streams.map((s) => (
              <button
                className={selectedStreamId === s.stream_id ? 'listItem selected' : 'listItem'}
                key={s.stream_id}
                onClick={() => setSelectedStreamId(s.stream_id)}
              >
                <strong>{s.title || '(untitled)'}</strong>
                <span>{s.stream_id}</span>
              </button>
            ))}
          </div>
          <label>Selected stream id</label>
          <input
            value={selectedStreamId}
            onChange={(e) => setSelectedStreamId(e.target.value)}
            placeholder="stream_id"
          />
        </div>

        <div className="panel">
          <h2>Create Stream</h2>
          <label>Type</label>
          <select
            value={createForm.stream_type}
            onChange={(e) => setCreateForm((p) => ({ ...p, stream_type: e.target.value }))}
          >
            <option value="broadcast">broadcast</option>
            <option value="private">private</option>
          </select>
          <label>Title</label>
          <input
            value={createForm.title}
            onChange={(e) => setCreateForm((p) => ({ ...p, title: e.target.value }))}
          />
          <label>Description</label>
          <input
            value={createForm.description}
            onChange={(e) => setCreateForm((p) => ({ ...p, description: e.target.value }))}
          />
          <label>Paid stream</label>
          <input
            type="checkbox"
            checked={createForm.is_paid}
            onChange={(e) => setCreateForm((p) => ({ ...p, is_paid: e.target.checked }))}
          />
          <label>Ticket price (USD)</label>
          <input
            type="number"
            min="0"
            step="0.01"
            value={createForm.ticket_price_usd}
            onChange={(e) => setCreateForm((p) => ({ ...p, ticket_price_usd: e.target.value }))}
          />
          <label>Invited viewer id (private only)</label>
          <input
            value={createForm.invited_viewer_id}
            onChange={(e) => setCreateForm((p) => ({ ...p, invited_viewer_id: e.target.value }))}
            placeholder="viewer user id"
          />
          <button onClick={createStream}>Create Stream</button>
        </div>

        <div className="panel">
          <h2>Playback + Ticket + Tip</h2>
          <button onClick={getPlayback}>Get Playback URL</button>
          <button onClick={purchaseTicket}>Purchase Ticket</button>
          <button onClick={verifyTicket}>Verify Ticket</button>
          <label>Tip amount</label>
          <input value={tipAmount} onChange={(e) => setTipAmount(e.target.value)} />
          <label>Tip message</label>
          <input value={tipMessage} onChange={(e) => setTipMessage(e.target.value)} />
          <button onClick={sendTip}>Send Tip</button>
          <label>Playback URL</label>
          <input value={playbackUrl} readOnly placeholder="Playback URL appears here" />
        </div>

        <div className="panel">
          <h2>Access Control</h2>
          <p style={{fontSize:'0.85em',color:'#888'}}>
            Locked (paid) streams → checks ticket ownership.<br/>
            Free streams → checks follow / subscribe via Laravel.
          </p>
          <button onClick={checkAccess}>Check My Access</button>
          {accessResult && (
            <pre style={{fontSize:'0.8em',background:'#111',padding:'8px',borderRadius:'6px',overflowX:'auto'}}>
              {JSON.stringify(accessResult, null, 2)}
            </pre>
          )}
          <button onClick={syncToLaravel}>Sync Stream → Laravel</button>
          {syncResult && (
            <pre style={{fontSize:'0.8em',background:'#111',padding:'8px',borderRadius:'6px',overflowX:'auto'}}>
              {JSON.stringify(syncResult, null, 2)}
            </pre>
          )}
        </div>
      </section>}

      {view === 'streams' && <section className="panel">
        <h2>Chat</h2>
        <div className="actions">
          <button onClick={connectChat}>Connect</button>
          <button onClick={disconnectChat}>Disconnect</button>
        </div>
        <div className="chatLog">
          {chatMessages.map((m, i) => (
            <div key={`${m.sent_at || ''}-${i}`} className="chatItem">
              <strong>{m.username || m.user_id || 'system'}</strong>
              <span>{m.body || JSON.stringify(m)}</span>
            </div>
          ))}
        </div>
        <div className="chatInput">
          <input
            value={chatInput}
            onChange={(e) => setChatInput(e.target.value)}
            placeholder="Type a message"
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                sendChatMessage();
              }
            }}
          />
          <button onClick={sendChatMessage}>Send</button>
        </div>
      </section>}

      {view === 'sessions' && <section className="grid">
        <div className="panel">
          <h2>Create Private Session</h2>
          <label>Title</label>
          <input
            value={createSessionForm.title}
            onChange={(e) => setCreateSessionForm((p) => ({ ...p, title: e.target.value }))}
          />
          <label>Description</label>
          <input
            value={createSessionForm.description}
            onChange={(e) => setCreateSessionForm((p) => ({ ...p, description: e.target.value }))}
          />
          <label>Invited viewer id</label>
          <input
            value={createSessionForm.invited_viewer_id}
            onChange={(e) => setCreateSessionForm((p) => ({ ...p, invited_viewer_id: e.target.value }))}
          />
          <label>Price (USD)</label>
          <input
            type="number"
            min="0"
            step="0.01"
            value={createSessionForm.price_usd}
            onChange={(e) => setCreateSessionForm((p) => ({ ...p, price_usd: e.target.value }))}
          />
          <button onClick={createSession}>Create Session</button>
        </div>

        <div className="panel">
          <h2>Invite + Manage Session</h2>
          <label>Session id</label>
          <input
            value={sessionActionId}
            onChange={(e) => setSessionActionId(e.target.value)}
            placeholder="session stream id"
          />
          <label>Viewer id for invite</label>
          <input
            value={sessionInviteViewerId}
            onChange={(e) => setSessionInviteViewerId(e.target.value)}
            placeholder="viewer user id"
          />
          <button onClick={inviteToSession}>Send Invite</button>
          <button onClick={() => acceptSession()}>Accept Session</button>
          <button onClick={() => declineSession()}>Decline Session</button>
        </div>

        <div className="panel">
          <h2>Incoming Session Invites</h2>
          <button onClick={loadIncomingSessions}>Refresh Incoming</button>
          <div className="list">
            {incomingSessions.map((s) => (
              <div className="sessionCard" key={s.stream_id}>
                <strong>{s.title || 'Private Session'}</strong>
                <span>{s.stream_id}</span>
                <div className="actions">
                  <button onClick={() => acceptSession(s.stream_id)}>Accept</button>
                  <button onClick={() => declineSession(s.stream_id)}>Decline</button>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>}

      <footer className="status">Status: {status}</footer>
    </div>
  );
}
