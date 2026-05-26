import { useEffect, useMemo, useRef, useState } from 'react';
import videojs from 'video.js';
import 'video.js/dist/video-js.css';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080/api/v1';
const STREAM_STATUS_POLL_MS = Number(import.meta.env.VITE_STREAM_STATUS_POLL_MS || 5000);

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

function normalizePlaybackUrl(url) {
  const raw = String(url || '').trim();
  if (!raw) {
    return '';
  }
  if (raw.includes('playback.local')) {
    return 'https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8';
  }
  return raw;
}

function isStreamNotFoundError(message) {
  return String(message || '').toLowerCase().includes('stream_not_found');
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
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
  const [ingestError, setIngestError] = useState('');

  const [singleViewerId, setSingleViewerId] = useState('viewer-1');
  const [singleViewerResult, setSingleViewerResult] = useState(null);

  const [multiViewerInput, setMultiViewerInput] = useState('viewer-2, viewer-3, viewer-4');
  const [multiViewerResults, setMultiViewerResults] = useState([]);
  const [recordingUrl, setRecordingUrl] = useState('');
  const [viewerPlaybackUrl, setViewerPlaybackUrl] = useState('');
  const [watchLiveError, setWatchLiveError] = useState('');
  const [viewerUseLocalPreview, setViewerUseLocalPreview] = useState(false);

  const [streamStatus, setStreamStatus] = useState('');
  const [lastStatusCheckAt, setLastStatusCheckAt] = useState('');
  const [statusPollError, setStatusPollError] = useState('');
  const [ivsIsLive, setIvsIsLive] = useState(null);
  const [ivsLastCheckedAt, setIvsLastCheckedAt] = useState('');
  const [ivsStatusError, setIvsStatusError] = useState('');
  const [autoPollStatus, setAutoPollStatus] = useState(true);
  const [autoRefreshOnLive, setAutoRefreshOnLive] = useState(true);

  const [activePlaybackUrl, setActivePlaybackUrl] = useState('');
  const [showCreatorCameraScreen, setShowCreatorCameraScreen] = useState(false);
  const [cameraError, setCameraError] = useState('');
  const [status, setStatus] = useState('Ready.');
  const liveTriggerRef = useRef(false);
  const creatorVideoRef = useRef(null);
  const creatorStreamRef = useRef(null);
  const viewerVideoRef = useRef(null);
  const broadcastClientRef = useRef(null);
  const isBrowserBroadcastingRef = useRef(false);

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

  async function createBroadcastAndReturn() {
    if (!creatorActor.user_id) {
      throw new Error('Set creator id first.');
    }

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

    return result.data || null;
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
    setIngestError('');
    setStreamStatus((stream?.status || '').toUpperCase());
    setStatusPollError('');
    setLastStatusCheckAt(new Date().toISOString());
    setIvsIsLive(null);
    setIvsLastCheckedAt('');
    setIvsStatusError('');
    liveTriggerRef.current = false;

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

      const ingestData = result.data || null;
      setIngestInfo(ingestData);
      setIngestError('');
      setStatus('Ingest credentials loaded. Start OBS broadcast now.');
      return ingestData;
    } catch (error) {
      setIngestError(error.message);
      setStatus(`Load ingest credentials failed: ${error.message}`);
      return null;
    }
  }

  async function loadIngestForSelected() {
    if (!selectedStream?.stream_id) {
      setStatus('Select a stream first.');
      return;
    }
    await selectAndLoadIngest(selectedStream);
  }

  async function startStreamingSetup() {
    try {
      setStatus('Preparing start streaming setup...');

      let stream = selectedStream;
      if (!stream?.stream_id) {
        stream = await createBroadcastAndReturn();
        setSelectedStream(stream);
        await loadCreatorStreams();
      }

      const ingest = await selectAndLoadIngest(stream);
      if (!ingest?.stream_key || !ingest?.ingest_endpoint) {
        throw new Error('ingest_credentials_unavailable');
      }

      setStatus('Start streaming ready. Camera + mic can now broadcast directly from browser with IVS SDK.');
      return { stream, ingest };
    } catch (error) {
      setStatus(`Start streaming setup failed: ${error.message}`);
      return null;
    }
  }

  async function ensureIvsBroadcastSdk(timeoutMs = 5000) {
    const start = Date.now();
    while (Date.now() - start < timeoutMs) {
      if (window?.IVSBroadcastClient?.create) {
        return window.IVSBroadcastClient;
      }
      await sleep(100);
    }
    throw new Error('ivs_broadcast_sdk_unavailable');
  }

  async function stopIvsBrowserBroadcast() {
    const client = broadcastClientRef.current;
    if (!client) {
      isBrowserBroadcastingRef.current = false;
      return;
    }

    try {
      if (typeof client.stopBroadcast === 'function') {
        await client.stopBroadcast();
      }
    } catch {
      // Ignore stop errors; cleanup continues.
    } finally {
      broadcastClientRef.current = null;
      isBrowserBroadcastingRef.current = false;
    }
  }

  async function startIvsBrowserBroadcast(ingestEndpoint, streamKey) {
    const sdk = await ensureIvsBroadcastSdk();
    const endpoint = String(ingestEndpoint || '').trim();
    const key = String(streamKey || '').trim();

    if (!endpoint || !key) {
      throw new Error('missing_ingest_credentials');
    }

    const media = creatorStreamRef.current;
    if (!media) {
      throw new Error('camera_stream_unavailable');
    }

    await stopIvsBrowserBroadcast();

    const streamConfig = sdk.BASIC_LANDSCAPE || sdk.STANDARD_LANDSCAPE || sdk.STANDARD_PORTRAIT;
    const client = sdk.create({
      streamConfig,
      ingestEndpoint: endpoint
    });

    const videoTrack = media.getVideoTracks()[0];
    const audioTrack = media.getAudioTracks()[0];

    if (videoTrack) {
      await client.addVideoInputDevice(media, 'camera', { index: 0 });
    }
    if (audioTrack) {
      await client.addAudioInputDevice(media, 'mic');
    }

    // NOTE: attachPreview requires a <canvas> element, not <video>.
    // Local camera preview is already handled via srcObject on the video ref.

    await client.startBroadcast(key);
    broadcastClientRef.current = client;
    isBrowserBroadcastingRef.current = true;
  }

  async function markStreamLiveWithRetry(streamId) {
    const maxAttempts = 5;
    let lastError = 'start_live_failed';

    for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
      const result = await requestWithStatus(`/streams/${streamId}/start-live`, { method: 'POST' }, creatorActor);
      if (result.ok) {
        return result;
      }

      const msg = result.error || `HTTP ${result.status}`;
      lastError = msg;
      if (!String(msg).includes('ivs_channel_not_live_start_encoder') || attempt === maxAttempts) {
        throw new Error(msg);
      }

      await sleep(1200);
    }

    throw new Error(lastError);
  }

  async function startLiveBroadcast() {
    try {
      setStatus('Starting live broadcast as creator...');

      const cameraReady = await openCreatorCameraScreen();
      if (!cameraReady) {
        setStatus('Camera access failed. Grant camera permission and try again.');
        return;
      }

      let stream = selectedStream;
      let ingest = ingestInfo;

      if (!stream?.stream_id || !ingest?.stream_key || !ingest?.ingest_endpoint) {
        const setup = await startStreamingSetup();
        if (!setup?.stream || !setup?.ingest) {
          return;
        }
        stream = setup.stream;
        ingest = setup.ingest;
      }

      const streamId = (stream?.stream_id || selectedStream?.stream_id || '').trim();
      if (!streamId) {
        setStatus('Could not determine stream id for live start.');
        return;
      }

      await startIvsBrowserBroadcast(ingest?.ingest_endpoint, ingest?.stream_key);
      setStatus('IVS browser broadcast started. Marking stream LIVE...');

      const result = await markStreamLiveWithRetry(streamId);

      const updated = result.data || null;
      if (updated) {
        setSelectedStream(updated);
      }
      setStreamStatus('LIVE');
      setIvsIsLive(true);
      setIvsLastCheckedAt(new Date().toISOString());
      setLastStatusCheckAt(new Date().toISOString());
      setStatus('Creator is LIVE and browser broadcast is active. Use Watch Live Broadcast for viewers.');
    } catch (error) {
      const message = String(error?.message || 'unknown_error');
      if (message.includes('ivs_channel_not_live_start_encoder')) {
        setIvsIsLive(false);
        setIvsLastCheckedAt(new Date().toISOString());
        setStatus('AWS IVS channel is not broadcasting yet. Ensure camera/mic permission is granted and retry Start Live Broadcast.');
        return;
      }
      if (message.includes('ivs_broadcast_sdk_unavailable')) {
        setStatus('Amazon IVS Web Broadcast SDK failed to load. Refresh the page and try again.');
        return;
      }
      setStatus(`Start live broadcast failed: ${message}`);
    }
  }

  async function openCreatorCameraScreen() {
    if (!navigator?.mediaDevices?.getUserMedia) {
      setCameraError('Browser does not support camera access.');
      setShowCreatorCameraScreen(true);
      return false;
    }

    try {
      if (!creatorStreamRef.current) {
        creatorStreamRef.current = await navigator.mediaDevices.getUserMedia({
          video: true,
          audio: true
        });
      }

      setCameraError('');
      setShowCreatorCameraScreen(true);
      return true;
    } catch (error) {
      setCameraError(error?.message || 'Could not access camera device.');
      setShowCreatorCameraScreen(true);
      return false;
    }
  }

  function stopCreatorCamera() {
    if (creatorStreamRef.current) {
      creatorStreamRef.current.getTracks().forEach((track) => track.stop());
      creatorStreamRef.current = null;
    }
    if (creatorVideoRef.current) {
      creatorVideoRef.current.srcObject = null;
    }
  }

  function closeCreatorCameraScreen() {
    stopCreatorCamera();
    setViewerUseLocalPreview(false);
    setShowCreatorCameraScreen(false);
    setCameraError('');
  }

  async function stopLiveBroadcast() {
    const streamId = selectedStream?.stream_id;
    if (!streamId) {
      await stopIvsBrowserBroadcast();
      setStatus('Select or create a stream first.');
      return;
    }

    try {
      setStatus('Stopping live broadcast as creator...');
      await stopIvsBrowserBroadcast();
      const result = await requestWithStatus(`/streams/${streamId}/stop-live`, { method: 'POST' }, creatorActor);
      if (!result.ok) {
        throw new Error(result.error || `HTTP ${result.status}`);
      }

      const updated = result.data || null;
      if (updated) {
        setSelectedStream(updated);
      }
      setStreamStatus('ENDED');
      setIvsIsLive(false);
      setIvsLastCheckedAt(new Date().toISOString());
      setLastStatusCheckAt(new Date().toISOString());
      setStatus('Creator broadcast stopped.');
    } catch (error) {
      setStatus(`Stop live broadcast failed: ${error.message}`);
    }
  }

  async function checkSelectedIvsStatus(options = {}) {
    const { silent = false } = options;
    const streamId = selectedStream?.stream_id;
    if (!streamId) {
      return null;
    }

    const result = await requestWithStatus(`/streams/${streamId}/ivs-status`, {}, creatorActor);
    const latest = result?.data || null;
    const nextCheckedAt = latest?.last_checked_at || new Date().toISOString();

    if (!result.ok) {
      const msg = result.error || `HTTP ${result.status}`;
      setIvsStatusError(msg);
      if (isStreamNotFoundError(msg)) {
        setSelectedStream(null);
        setStreamStatus('');
        setIvsIsLive(null);
        setIvsLastCheckedAt('');
        setStatusPollError('');
        setStatus('Selected stream was not found. Reload or create a broadcast channel again.');
        return null;
      }
      if (!silent) {
        setStatus(`IVS status check failed: ${msg}`);
      }
      return null;
    }

    setIvsIsLive(Boolean(latest?.is_live));
    setIvsLastCheckedAt(nextCheckedAt);
    setIvsStatusError('');
    if (!silent) {
      setStatus(latest?.is_live ? 'IVS channel is LIVE and receiving ingest.' : 'IVS channel is IDLE. Start OBS/encoder first.');
    }

    return latest;
  }

  async function watchLiveBroadcast() {
    const streamId = selectedStream?.stream_id;
    const viewerId = singleViewerId.trim() || 'viewer-1';

    if (!streamId) {
      setStatus('Select or create a stream first.');
      return;
    }

    try {
      setWatchLiveError('');

      if (creatorStreamRef.current) {
        setViewerUseLocalPreview(true);
        setViewerPlaybackUrl('local-device-preview');
        setStatus('User watch view connected to creator local camera preview.');
        return;
      }

      setViewerUseLocalPreview(false);
      setStatus(`Loading live playback for viewer ${viewerId}...`);
      const report = await runViewerFlow(streamId, viewerId);
      setSingleViewerResult(report);

      const playback = normalizePlaybackUrl(report.playback_url);
      if (!playback) {
        setWatchLiveError('Playback URL is empty. Creator may not be live yet.');
        setStatus('No playback URL available for viewer.');
        return;
      }

      if (streamStatus !== 'LIVE') {
        setWatchLiveError('Stream is not LIVE yet. Click Start Live Broadcast first.');
      }

      setViewerPlaybackUrl(playback);
      setActivePlaybackUrl(playback);
      setStatus('User watch player loaded.');
    } catch (error) {
      setWatchLiveError(error.message);
      setStatus(`Watch live broadcast failed: ${error.message}`);
    }
  }

  async function checkSelectedStreamStatus(options = {}) {
    const { silent = false } = options;
    const streamId = selectedStream?.stream_id;
    if (!streamId) {
      return null;
    }

    const result = await requestWithStatus(`/streams/${streamId}`, {}, creatorActor);
    const latest = result?.data || result?.raw || null;
    const normalizedStatus = String(latest?.status || '').toUpperCase();

    if (!result.ok) {
      const msg = result.error || `HTTP ${result.status}`;
      setStatusPollError(msg);
      if (isStreamNotFoundError(msg)) {
        setSelectedStream(null);
        setStreamStatus('');
        setIvsIsLive(null);
        setIvsLastCheckedAt('');
        setIvsStatusError('');
        setStatus('Selected stream was not found. Reload or create a broadcast channel again.');
        return null;
      }
      if (!silent) {
        setStatus(`Status check failed: ${msg}`);
      }
      return null;
    }

    setSelectedStream(latest);
    setStreamStatus(normalizedStatus);
    setStatusPollError('');
    setLastStatusCheckAt(new Date().toISOString());

    if (normalizedStatus === 'LIVE') {
      if (autoRefreshOnLive && !liveTriggerRef.current) {
        liveTriggerRef.current = true;
        if (!silent) {
          setStatus('Stream is LIVE. Auto-running one-viewer refresh.');
        }
        await watchAsOne();
      } else if (!silent) {
        setStatus('Stream is LIVE.');
      }
    } else {
      liveTriggerRef.current = false;
      if (!silent) {
        setStatus(`Stream status: ${normalizedStatus || 'unknown'}.`);
      }
    }

    return latest;
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
    report.playback_url = normalizePlaybackUrl(playbackResult?.data?.playback_url || '');
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
        setViewerPlaybackUrl(report.playback_url);
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
        setViewerPlaybackUrl(firstPlayable.playback_url);
        setActivePlaybackUrl(firstPlayable.playback_url);
      }

      const passedCount = reports.filter((item) => item.ok).length;
      setStatus(`1-to-many complete: ${passedCount}/${reports.length} viewer(s) can watch.`);
    } catch (error) {
      setStatus(`1-to-many flow failed: ${error.message}`);
    }
  }

  async function getRecording() {
    const streamId = selectedStream?.stream_id;
    const viewerId = singleViewerId.trim();
    if (!streamId) {
      setStatus('Select a stream first.');
      return;
    }
    if (!viewerId) {
      setStatus('Enter viewer id first to get recording.');
      return;
    }

    try {
      setStatus('Fetching recording URL...');
      const viewer = asActor(viewerId, 'viewer', viewerId);
      const result = await requestWithStatus(`/streams/${streamId}/recording`, {}, viewer);
      if (!result.ok) {
        throw new Error(result.error || `HTTP ${result.status}`);
      }

      const url = result?.data?.recording_url || '';
      setRecordingUrl(url);
      if (url) {
        setActivePlaybackUrl(url);
      }
      setStatus(url ? 'Recording URL loaded.' : 'Recording response returned without URL.');
    } catch (error) {
      setStatus(`Get recording failed: ${error.message}`);
    }
  }

  async function launchViewerTab() {
    const streamId = selectedStream?.stream_id;
    if (!streamId) {
      setStatus('Select a stream first.');
      return;
    }

    let url = activePlaybackUrl;

    if (!url) {
      const report = await runViewerFlow(streamId, singleViewerId.trim() || 'viewer-1');
      setSingleViewerResult(report);
      url = report.playback_url || '';
      if (url) {
        setViewerPlaybackUrl(url);
        setActivePlaybackUrl(url);
      }
    }

    if (!url) {
      setStatus('No playback URL available to open in new tab.');
      return;
    }

    window.open(url, '_blank', 'noopener,noreferrer');
    setStatus('Opened viewer tab.');
  }

  useEffect(() => {
    const streamId = selectedStream?.stream_id;
    if (!streamId || !autoPollStatus) {
      return undefined;
    }

    void checkSelectedStreamStatus({ silent: true });
    void checkSelectedIvsStatus({ silent: true });
    const timer = setInterval(() => {
      void checkSelectedStreamStatus({ silent: true });
      void checkSelectedIvsStatus({ silent: true });
    }, STREAM_STATUS_POLL_MS);

    return () => clearInterval(timer);
  }, [selectedStream?.stream_id, autoPollStatus, creatorActor.user_id, token, autoRefreshOnLive]);

  useEffect(() => {
    if (!creatorVideoRef.current || !creatorStreamRef.current) {
      return;
    }

    creatorVideoRef.current.srcObject = creatorStreamRef.current;
    creatorVideoRef.current.play().catch(() => {
      // autoplay can be blocked by browser policy; controls are still visible.
    });
  }, [showCreatorCameraScreen, creatorStreamRef.current]);

  useEffect(() => {
    if (!viewerUseLocalPreview || !viewerVideoRef.current || !creatorStreamRef.current) {
      return;
    }

    viewerVideoRef.current.srcObject = creatorStreamRef.current;
    viewerVideoRef.current.play().catch(() => {
      // autoplay can be blocked by browser policy.
    });
  }, [viewerUseLocalPreview, creatorStreamRef.current]);

  useEffect(() => {
    return () => {
      void stopIvsBrowserBroadcast();
      stopCreatorCamera();
    };
  }, []);

  const checklist = [
    {
      key: 'created',
      label: 'Broadcast channel created',
      done: Boolean(selectedStream?.stream_id)
    },
    {
      key: 'ingest',
      label: 'OBS ingest credentials loaded',
      done: Boolean(ingestInfo?.ingest_endpoint && ingestInfo?.stream_key)
    },
    {
      key: 'live',
      label: 'Creator is LIVE',
      done: streamStatus === 'LIVE'
    },
    {
      key: 'ivs-live',
      label: 'AWS IVS channel receiving ingest',
      done: ivsIsLive === true
    }
  ];

  return (
    <div className="page">
      <header className="hero">
        <h1>Broadcast Channel Console</h1>
        <p>Create a livestream channel, start OBS broadcast, and validate one or many viewers.</p>
      </header>

      <section className="panel">
        <h2>Live Studio (Side By Side)</h2>
        <p className="status">Creator broadcast view is on the left, user watch view is on the right.</p>
        <div className="studioGrid">
          <div className="studioCard creatorStudio">
            <h3>Creator Broadcast View</h3>
            {cameraError && <p className="statusWarn">Camera error: {cameraError}</p>}
            <video ref={creatorVideoRef} className="creatorCameraVideo" autoPlay muted playsInline controls />
            <div className="cameraActions">
              <button className="primaryAction" onClick={openCreatorCameraScreen}>Enable Camera</button>
              <button className="creatorAction" onClick={startLiveBroadcast}>Start Live Broadcast</button>
              <button className="stopAction" onClick={stopLiveBroadcast}>Stop Live Broadcast</button>
            </div>
          </div>

          <div className="studioCard viewerStudio">
            <h3>User Watch View</h3>
            {watchLiveError && <p className="statusWarn">Watch error: {watchLiveError}</p>}
            <label>User playback URL</label>
            <input value={viewerPlaybackUrl} readOnly placeholder="Click Watch Live Broadcast" />
            {viewerUseLocalPreview ? (
              <video ref={viewerVideoRef} className="creatorCameraVideo" autoPlay muted playsInline controls />
            ) : (
              <VideoPlayer src={viewerPlaybackUrl || activePlaybackUrl} />
            )}
            <div className="cameraActions">
              <button className="viewerAction" onClick={watchLiveBroadcast}>Watch Live Broadcast</button>
              <button onClick={launchViewerTab}>Open In New Tab</button>
            </div>
          </div>
        </div>
      </section>

      <section className="panel">
        <h2>Start Broadcast Checklist</h2>
        <div className="checklistMeta">
          <span className={`statusPill ${streamStatus === 'LIVE' ? 'live' : 'idle'}`}>
            Stream: {streamStatus || 'UNKNOWN'}
          </span>
          <span className={`statusPill ${ivsIsLive === true ? 'live' : ivsIsLive === false ? 'idle' : 'unknown'}`}>
            IVS: {ivsIsLive === null ? 'UNKNOWN' : ivsIsLive ? 'LIVE' : 'IDLE'}
          </span>
          <span>Poll every {STREAM_STATUS_POLL_MS} ms</span>
          <span>Last check: {lastStatusCheckAt || '-'}</span>
          <span>IVS checked: {ivsLastCheckedAt || '-'}</span>
        </div>
        {statusPollError && <p className="statusWarn">Status error: {statusPollError}</p>}
        {ivsStatusError && <p className="statusWarn">IVS status error: {ivsStatusError}</p>}
        <div className="checklistActions">
          <button onClick={() => checkSelectedStreamStatus({ silent: false })}>Check Status Now</button>
          <button onClick={() => checkSelectedIvsStatus({ silent: false })}>Check IVS Channel</button>
          <label className="inlineCheck" htmlFor="auto-poll-status">
            <input
              id="auto-poll-status"
              type="checkbox"
              checked={autoPollStatus}
              onChange={(event) => setAutoPollStatus(event.target.checked)}
            />
            Auto-poll stream status
          </label>
          <label className="inlineCheck" htmlFor="auto-refresh-live">
            <input
              id="auto-refresh-live"
              type="checkbox"
              checked={autoRefreshOnLive}
              onChange={(event) => setAutoRefreshOnLive(event.target.checked)}
            />
            Auto-refresh viewer when LIVE
          </label>
        </div>
        <div className="checklistList">
          {checklist.map((item) => (
            <div key={item.key} className={item.done ? 'checkItem done' : 'checkItem'}>
              <strong>{item.done ? 'DONE' : 'TODO'}</strong>
              <span>{item.label}</span>
            </div>
          ))}
        </div>
      </section>


      <footer className="status">Status: {status}</footer>
    </div>
  );
}
