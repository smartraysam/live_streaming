import { useEffect, useMemo, useRef, useState } from 'react';
import videojs from 'video.js';
import 'video.js/dist/video-js.css';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080/api/v1';

function parseJsonSafe(text) {
  try {
    return JSON.parse(text);
  } catch {
    return { raw: text };
  }
}

function VideoPlayer({ src }) {
  const containerRef = useRef(null);
  const playerRef = useRef(null);

  useEffect(() => {
    if (!containerRef.current || playerRef.current) {
      return;
    }

    const videoEl = document.createElement('video-js');
    videoEl.className = 'video-js vjs-default-skin vjs-big-play-centered';
    containerRef.current.appendChild(videoEl);

    playerRef.current = videojs(videoEl, {
      controls: true,
      fluid: true,
      responsive: true,
      preload: 'auto'
    });
  }, []);

  useEffect(() => {
    if (!playerRef.current) {
      return;
    }

    if (!src) {
      playerRef.current.pause();
      playerRef.current.reset();
      return;
    }

    playerRef.current.src({ src, type: 'application/x-mpegURL' });
    playerRef.current.load();
  }, [src]);

  useEffect(() => {
    return () => {
      if (playerRef.current) {
        playerRef.current.dispose();
        playerRef.current = null;
      }
    };
  }, []);

  return <div data-vjs-player ref={containerRef} className="videoPlayerHost" />;
}

function toRtmpsServer(ingestEndpoint) {
  if (!ingestEndpoint) {
    return '';
  }
  return `rtmps://${ingestEndpoint}:443/app/`;
}

function asActor(userId, role, username) {
  return {
    user_id: String(userId || '').trim(),
    role: role || 'viewer',
    username: String(username || '').trim()
  };
}

export default function App() {
  const [token, setToken] = useState('');
  const [creatorId, setCreatorId] = useState('creator-1');
  const [creatorName, setCreatorName] = useState('Creator');

  const [streamForm, setStreamForm] = useState({
    title: 'Broadcast Test Stream',
    description: 'Live channel created from React app',
    is_paid: false,
    ticket_price_usd: 0
  });

  const [creatorStreams, setCreatorStreams] = useState([]);
  const [selectedStream, setSelectedStream] = useState(null);
  const [ingestInfo, setIngestInfo] = useState(null);

  const [singleViewerId, setSingleViewerId] = useState('viewer-1');
  const [singleViewerResult, setSingleViewerResult] = useState(null);

  const [multiViewerInput, setMultiViewerInput] = useState('viewer-2, viewer-3, viewer-4');
  const [multiViewerResults, setMultiViewerResults] = useState([]);

  const [activePlaybackUrl, setActivePlaybackUrl] = useState('');
  const [status, setStatus] = useState('Ready.');

  const creatorActor = useMemo(
    () => asActor(creatorId, 'creator', creatorName || creatorId),
    [creatorId, creatorName]
  );

  async function requestWithStatus(path, options = {}, actor = null) {
    const headers = { ...(options.headers || {}) };
    if (token.trim()) {
      headers.Authorization = `Bearer ${token.trim()}`;
    }
    if (actor?.user_id) {
      headers['X-User-ID'] = actor.user_id;
    }
    if (actor?.role) {
      headers['X-User-Role'] = actor.role;
    }
    if (actor?.username) {
      headers['X-Username'] = actor.username;
    }
    if (options.body) {
      headers['Content-Type'] = 'application/json';
    }

    const response = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers
    });

    const text = await response.text();
    const json = parseJsonSafe(text);

    return {
      ok: response.ok,
      status: response.status,
      data: json?.data,
      error: json?.error,
      raw: json
    };
  }

  async function createBroadcastChannel() {
    if (!creatorActor.user_id) {
      setStatus('Set creator id first.');
      return;
    }

    try {
      setStatus('Creating broadcast stream channel...');
      const payload = {
        stream_type: 'broadcast',
        title: streamForm.title,
        description: streamForm.description,
        is_paid: Boolean(streamForm.is_paid),
        ticket_price_usd: Number(streamForm.ticket_price_usd || 0)
      };

      const result = await requestWithStatus('/streams', {
        method: 'POST',
        body: JSON.stringify(payload)
      }, creatorActor);

      if (!result.ok) {
        throw new Error(result.error || `HTTP ${result.status}`);
      }

      setSelectedStream(result.data || null);
      setStatus(`Broadcast channel created: ${result?.data?.stream_id || 'ok'}`);
      await loadCreatorStreams();
    } catch (error) {
      setStatus(`Create broadcast failed: ${error.message}`);
    }
  }

  async function loadCreatorStreams() {
    if (!creatorActor.user_id) {
      setStatus('Set creator id first.');
      return;
    }

    try {
      setStatus(`Loading streams for creator ${creatorActor.user_id}...`);
      const result = await requestWithStatus(
        `/streams/creator/${encodeURIComponent(creatorActor.user_id)}`,
        {},
        creatorActor
      );

      if (!result.ok) {
        throw new Error(result.error || `HTTP ${result.status}`);
      }

      const items = Array.isArray(result.data) ? result.data : [];
      const broadcastOnly = items.filter((item) => item?.stream_type === 'broadcast');
      setCreatorStreams(broadcastOnly);
      setStatus(`Loaded ${broadcastOnly.length} broadcast stream(s).`);
    } catch (error) {
      setStatus(`Load creator streams failed: ${error.message}`);
    }
  }

  async function selectAndLoadIngest(stream) {
    setSelectedStream(stream);
    setIngestInfo(null);

    try {
      setStatus('Loading OBS ingest credentials...');
      const result = await requestWithStatus(
        `/streams/${stream.stream_id}/ingest-info`,
        {},
        creatorActor
      );

      if (!result.ok) {
        throw new Error(result.error || `HTTP ${result.status}`);
      }

      setIngestInfo(result.data || null);
      setStatus('Ingest credentials loaded. Start OBS broadcast now.');
    } catch (error) {
      setStatus(`Load ingest credentials failed: ${error.message}`);
    }
  }

  async function runViewerFlow(streamId, viewerId) {
    const viewer = asActor(viewerId, 'viewer', viewerId);
    const report = {
      stream_id: streamId,
      viewer_id: viewer.user_id,
      steps: []
    };

    const accessResult = await requestWithStatus(`/streams/${streamId}/access`, {}, viewer);
    report.steps.push({
      step: 'access_check',
      ok: accessResult.ok,
      status: accessResult.status,
      result: accessResult.data || accessResult.raw
    });

    if (!accessResult.ok) {
      const purchaseResult = await requestWithStatus(
        `/streams/${streamId}/ticket/purchase`,
        { method: 'POST' },
        viewer
      );
      report.steps.push({
        step: 'ticket_purchase',
        ok: purchaseResult.ok,
        status: purchaseResult.status,
        result: purchaseResult.data || purchaseResult.raw
      });

      const verifyResult = await requestWithStatus(
        `/streams/${streamId}/ticket/verify`,
        {},
        viewer
      );
      report.steps.push({
        step: 'ticket_verify',
        ok: verifyResult.ok,
        status: verifyResult.status,
        result: verifyResult.data || verifyResult.raw
      });
    }

    const playbackResult = await requestWithStatus(`/streams/${streamId}/playback`, {}, viewer);
    report.steps.push({
      step: 'playback_fetch',
      ok: playbackResult.ok,
      status: playbackResult.status,
      result: playbackResult.data || playbackResult.raw
    });

    report.ok = playbackResult.ok;
    report.playback_url = playbackResult?.data?.playback_url || '';
    return report;
  }

  async function watchAsOne() {
    const streamId = selectedStream?.stream_id;
    const viewerId = singleViewerId.trim();

    if (!streamId) {
      setStatus('Select a stream first.');
      return;
    }
    if (!viewerId) {
      setStatus('Enter single viewer id first.');
      return;
    }

    try {
      setStatus(`Running watch flow for ${viewerId}...`);
      const report = await runViewerFlow(streamId, viewerId);
      setSingleViewerResult(report);
      if (report.playback_url) {
        setActivePlaybackUrl(report.playback_url);
      }
      setStatus(report.ok ? 'Single viewer can watch the live stream.' : 'Single viewer flow returned errors.');
    } catch (error) {
      setStatus(`Single viewer flow failed: ${error.message}`);
    }
  }

  async function watchAsMany() {
    const streamId = selectedStream?.stream_id;
    if (!streamId) {
      setStatus('Select a stream first.');
      return;
    }

    const viewers = Array.from(new Set(
      multiViewerInput
        .split(/[\n,]+/)
        .map((item) => item.trim())
        .filter(Boolean)
    ));

    if (viewers.length === 0) {
      setStatus('Enter at least one viewer id for 1-to-many test.');
      return;
    }

    try {
      setStatus(`Running 1-to-many watch flow for ${viewers.length} viewers...`);
      const reports = [];

      for (let i = 0; i < viewers.length; i += 1) {
        const viewerId = viewers[i];
        setStatus(`Checking viewer ${i + 1}/${viewers.length}: ${viewerId}`);
        const report = await runViewerFlow(streamId, viewerId);
        reports.push(report);
      }

      setMultiViewerResults(reports);

      const firstPlayable = reports.find((item) => item.playback_url);
      if (firstPlayable?.playback_url) {
        setActivePlaybackUrl(firstPlayable.playback_url);
      }

      const passedCount = reports.filter((item) => item.ok).length;
      setStatus(`1-to-many complete: ${passedCount}/${reports.length} viewer(s) can watch.`);
    } catch (error) {
      setStatus(`1-to-many flow failed: ${error.message}`);
    }
  }

  return (
    <div className="page">
      <header className="hero">
        <h1>Broadcast Channel Console</h1>
        <p>Create a livestream channel, start OBS broadcast, and validate one or many viewers.</p>
      </header>

      <section className="panel">
        <h2>Connection + Creator Identity</h2>
        <label>JWT token (if ENABLE_AUTH=true)</label>
        <input
          value={token}
          onChange={(event) => setToken(event.target.value)}
          placeholder="Paste bearer token"
        />
        <label>Creator user id</label>
        <input
          value={creatorId}
          onChange={(event) => setCreatorId(event.target.value)}
          placeholder="creator-1"
        />
        <label>Creator username</label>
        <input
          value={creatorName}
          onChange={(event) => setCreatorName(event.target.value)}
          placeholder="creator name"
        />
        <small>API base: {API_BASE}</small>
      </section>

      <section className="grid two">
        <div className="panel">
          <h2>1. Create Broadcast Channel</h2>
          <label>Title</label>
          <input
            value={streamForm.title}
            onChange={(event) => setStreamForm((prev) => ({ ...prev, title: event.target.value }))}
          />

          <label>Description</label>
          <input
            value={streamForm.description}
            onChange={(event) => setStreamForm((prev) => ({ ...prev, description: event.target.value }))}
          />

          <div className="inlineRow">
            <label className="inlineCheck" htmlFor="is-paid">
              <input
                id="is-paid"
                type="checkbox"
                checked={streamForm.is_paid}
                onChange={(event) => setStreamForm((prev) => ({ ...prev, is_paid: event.target.checked }))}
              />
              Paid stream
            </label>
          </div>

          <label>Ticket price (USD)</label>
          <input
            type="number"
            min="0"
            step="0.01"
            value={streamForm.ticket_price_usd}
            onChange={(event) => setStreamForm((prev) => ({ ...prev, ticket_price_usd: event.target.value }))}
          />

          <button onClick={createBroadcastChannel}>Create Broadcast</button>
          <button onClick={loadCreatorStreams}>Load My Broadcasts</button>
        </div>

        <div className="panel">
          <h2>2. Choose Channel + Ingest</h2>
          <div className="list">
            {creatorStreams.map((stream) => (
              <button
                key={stream.stream_id}
                className={selectedStream?.stream_id === stream.stream_id ? 'listItem selected' : 'listItem'}
                onClick={() => selectAndLoadIngest(stream)}
              >
                <strong>{stream.title || '(untitled)'}</strong>
                <span>{stream.stream_id}</span>
                <span>status: {stream.status}</span>
              </button>
            ))}
          </div>

          <label>Selected stream id</label>
          <input value={selectedStream?.stream_id || ''} readOnly placeholder="Select a stream" />

          <label>Ingest endpoint</label>
          <input value={ingestInfo?.ingest_endpoint || ''} readOnly placeholder="Load ingest info" />

          <label>OBS server URL</label>
          <input value={toRtmpsServer(ingestInfo?.ingest_endpoint)} readOnly placeholder="rtmps://.../app/" />

          <label>Stream key</label>
          <input value={ingestInfo?.stream_key || ''} readOnly placeholder="Load ingest info" />
        </div>
      </section>

      <section className="grid two">
        <div className="panel">
          <h2>3A. Watch As One Viewer</h2>
          <label>Viewer id</label>
          <input
            value={singleViewerId}
            onChange={(event) => setSingleViewerId(event.target.value)}
            placeholder="viewer-1"
          />
          <button onClick={watchAsOne}>Run 1-to-1 Watch Flow</button>
          {singleViewerResult && (
            <pre className="jsonReport">{JSON.stringify(singleViewerResult, null, 2)}</pre>
          )}
        </div>

        <div className="panel">
          <h2>3B. Watch As Many Viewers</h2>
          <label>Viewer ids (comma or newline separated)</label>
          <textarea
            value={multiViewerInput}
            onChange={(event) => setMultiViewerInput(event.target.value)}
            rows={5}
            placeholder="viewer-2, viewer-3, viewer-4"
          />
          <button onClick={watchAsMany}>Run 1-to-Many Watch Flow</button>
          {multiViewerResults.length > 0 && (
            <div className="reportList">
              {multiViewerResults.map((result) => (
                <div key={result.viewer_id} className={result.ok ? 'resultOk' : 'resultFail'}>
                  <strong>{result.viewer_id}</strong>
                  <span>{result.ok ? 'can watch' : 'cannot watch'}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      <section className="panel">
        <h2>Live Player Preview</h2>
        <label>Playback URL</label>
        <input value={activePlaybackUrl} readOnly placeholder="Run a watch flow first" />
        <VideoPlayer src={activePlaybackUrl} />
      </section>

      <footer className="status">Status: {status}</footer>
    </div>
  );
}
