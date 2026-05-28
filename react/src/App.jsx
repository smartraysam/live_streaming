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
  const videoRef = useRef(null);     // <video> element used by IVS Player
  const containerRef = useRef(null); // fallback container used by Video.js
  const playerRef = useRef(null);    // { type: 'ivs'|'vjs', p: playerInstance }

  // Init player once on mount
  useEffect(() => {
    const ivs = window.IVSPlayer;
    if (ivs && ivs.isPlayerSupported && videoRef.current) {
      // Amazon IVS Player: auto-uses Low-Latency HLS (2-5s lag vs 15-30s with HLS.js)
      const p = ivs.create();
      p.attachHTMLVideoElement(videoRef.current);
      playerRef.current = { type: 'ivs', p };
    } else if (containerRef.current && !playerRef.current) {
      // Fallback: Video.js with low-latency HLS.js config
      const videoEl = document.createElement('video-js');
      videoEl.className = 'video-js vjs-default-skin vjs-big-play-centered';
      containerRef.current.appendChild(videoEl);
      const p = videojs(videoEl, {
        controls: true,
        fluid: true,
        responsive: true,
        preload: 'auto',
        liveui: true,
        html5: {
          vhs: {
            overrideNative: true,
            enableLowInitialPlaylist: true,
            liveSyncDurationCount: 2,
            liveMaxLatencyDurationCount: 4,
          },
        },
      });
      playerRef.current = { type: 'vjs', p };
    }
    return () => {
      if (!playerRef.current) return;
      if (playerRef.current.type === 'ivs') playerRef.current.p.delete();
      else playerRef.current.p.dispose();
      playerRef.current = null;
    };
  }, []);

  // Load / change source
  useEffect(() => {
    const ref = playerRef.current;
    if (!ref) return;
    if (!src) {
      ref.p.pause();
      if (ref.type === 'vjs') ref.p.reset();
      return;
    }
    if (ref.type === 'ivs') {
      ref.p.load(src);
      ref.p.play();
    } else {
      ref.p.src({ src, type: 'application/x-mpegURL' });
      ref.p.load();
    }
  }, [src]);

  const ivsReady = !!(window.IVSPlayer && window.IVSPlayer.isPlayerSupported);

  return (
    <div data-vjs-player ref={containerRef} className="videoPlayerHost">
      {/* IVS Player uses a plain <video> element */}
      <video
        ref={videoRef}
        style={{ width: '100%', height: '100%', display: ivsReady ? 'block' : 'none' }}
        controls
        playsInline
      />
    </div>
  );
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

// ──────────────────────────────────────────────────────────────────────────────
// StageParticipantView – renders one remote participant's video/audio
// ──────────────────────────────────────────────────────────────────────────────
function StageParticipantView({ participantId, streams, muted = false }) {
  const videoRef = useRef(null);

  useEffect(() => {
    if (!videoRef.current || !streams || streams.length === 0) return;

    const videoTrack = streams.find((s) => s.streamType === 'video' || s.mediaStreamTrack?.kind === 'video');
    const audioTrack = streams.find((s) => s.streamType === 'audio' || s.mediaStreamTrack?.kind === 'audio');

    const tracks = [];
    if (videoTrack?.mediaStreamTrack) tracks.push(videoTrack.mediaStreamTrack);
    if (audioTrack?.mediaStreamTrack) tracks.push(audioTrack.mediaStreamTrack);

    if (tracks.length > 0) {
      videoRef.current.srcObject = new MediaStream(tracks);
      videoRef.current.play().catch(() => {});
    }
    return () => {
      if (videoRef.current) videoRef.current.srcObject = null;
    };
  }, [streams]);

  return (
    <div className="stageParticipant">
      <p className="participantLabel">{participantId}</p>
      <video ref={videoRef} autoPlay playsInline muted={muted} style={{ width: '100%', borderRadius: 8, background: '#000' }} />
    </div>
  );
}

function applyTrackState(mediaStream, { videoEnabled = true, audioEnabled = true }) {
  if (!mediaStream) {
    return;
  }
  mediaStream.getVideoTracks().forEach((track) => {
    track.enabled = videoEnabled;
  });
  mediaStream.getAudioTracks().forEach((track) => {
    track.enabled = audioEnabled;
  });
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
  const rtCreatorVideoRef = useRef(null);
  const creatorStreamRef = useRef(null);
  const rtGuestVideoRef = useRef(null);
  const guestStreamRef = useRef(null);
  const viewerVideoRef = useRef(null);
  const broadcastClientRef = useRef(null);
  const isBrowserBroadcastingRef = useRef(false);

  // ── IVS Real-Time (Stages) state ──────────────────────────────────────────
  const [stageMode, setStageMode] = useState('CALL');
  const [stageTitle, setStageTitle] = useState('Quick call');
  const [currentStage, setCurrentStage] = useState(null);   // backend Stage object
  const [stageConnState, setStageConnState] = useState('');  // 'connecting'|'connected'|'disconnected'
  const [stageParticipants, setStageParticipants] = useState({}); // { [pid]: streams[] }
  const [rtViewerId, setRtViewerId] = useState('guest-rt-1');
  const [rtViewerName, setRtViewerName] = useState('Guest');
  const [viewerStageConnState, setViewerStageConnState] = useState('');
  const [viewerStageParticipants, setViewerStageParticipants] = useState({});
  const [rtCreatorSettings, setRtCreatorSettings] = useState({
    muteAllGuests: false,
    disableStageChat: false,
    cameraEnabled: true,
    muted: false,
    overrideChatLock: true
  });
  const [rtGuestSettings, setRtGuestSettings] = useState({
    muted: false,
    cameraEnabled: true,
    overrideCreatorSettings: false,
    speakerMuted: false
  });
  const [rtGuestCameraError, setRtGuestCameraError] = useState('');
  const [rtStageChatMessages, setRtStageChatMessages] = useState([]);
  const [rtCreatorChatInput, setRtCreatorChatInput] = useState('');
  const [rtGuestChatInput, setRtGuestChatInput] = useState('');
  const [stageError, setStageError] = useState('');
  const stageClientRef = useRef(null);  // IVS Stage SDK instance
  const stageViewerClientRef = useRef(null);

  const creatorActor = useMemo(
    () => asActor(creatorId, 'creator', creatorName || creatorId),
    [creatorId, creatorName]
  );

  const canGuestPublishVideo = currentStage?.mode === 'CALL' && rtGuestSettings.cameraEnabled;
  const canGuestPublishAudio = currentStage?.mode === 'CALL' && !rtGuestSettings.muted && (!rtCreatorSettings.muteAllGuests || rtGuestSettings.overrideCreatorSettings);
  const isCreatorChatDisabled = rtCreatorSettings.disableStageChat && !rtCreatorSettings.overrideChatLock;
  const isGuestChatDisabled = rtCreatorSettings.disableStageChat && !rtGuestSettings.overrideCreatorSettings;

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
    if (rtCreatorVideoRef.current) {
      rtCreatorVideoRef.current.srcObject = null;
    }
  }

  async function openGuestCameraScreen() {
    if (!navigator?.mediaDevices?.getUserMedia) {
      setRtGuestCameraError('Browser does not support camera access.');
      return false;
    }

    try {
      if (!guestStreamRef.current) {
        guestStreamRef.current = await navigator.mediaDevices.getUserMedia({
          video: true,
          audio: true
        });
      }
      setRtGuestCameraError('');
      return true;
    } catch (error) {
      setRtGuestCameraError(error?.message || 'Could not access guest camera device.');
      return false;
    }
  }

  function stopGuestCamera() {
    if (guestStreamRef.current) {
      guestStreamRef.current.getTracks().forEach((track) => track.stop());
      guestStreamRef.current = null;
    }
    if (rtGuestVideoRef.current) {
      rtGuestVideoRef.current.srcObject = null;
    }
  }

  function sendRTStageChatMessage(author, body) {
    const text = String(body || '').trim();
    if (!text) {
      return;
    }
    setRtStageChatMessages((prev) => ([
      ...prev,
      {
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        author,
        body: text,
        createdAt: new Date().toISOString()
      }
    ]));
  }

  function closeCreatorCameraScreen() {
    stopCreatorCamera();
    setViewerUseLocalPreview(false);
    setShowCreatorCameraScreen(false);
    setCameraError('');
  }

  // ── IVS Real-Time (Stages) functions ────────────────────────────────────────

  async function createRTStage() {
    if (!creatorActor.user_id) { setStageError('Set creator id first.'); return; }
    if (!stageTitle.trim()) { setStageError('Enter a stage title.'); return; }
    try {
      setStageError('');
      setStatus('Creating real-time stage...');
      const result = await requestWithStatus('/stages', {
        method: 'POST',
        body: JSON.stringify({ mode: stageMode, title: stageTitle.trim() })
      }, creatorActor);
      if (!result.ok) throw new Error(result.error || `HTTP ${result.status}`);
      setCurrentStage(result.data);
      setStageParticipants({});
      setViewerStageParticipants({});
      setRtStageChatMessages([]);
      setStageConnState('');
      setViewerStageConnState('');
      setStatus(`Stage "${result.data.title}" created. Click Join to connect.`);
    } catch (e) {
      setStageError(e.message);
      setStatus(`Create stage failed: ${e.message}`);
    }
  }

  function attachStageEventHandlers(stage, sdk, setConnState, setParticipants) {
    stage.on(sdk.StageEvents.STAGE_CONNECTION_STATE_CHANGED, (state) => {
      const label = String(state).toLowerCase();
      setConnState(label);
    });

    stage.on(sdk.StageEvents.STAGE_PARTICIPANT_JOINED, (participant) => {
      if (participant.isLocal) return;
      setParticipants((prev) => ({ ...prev, [participant.id]: [] }));
    });

    stage.on(sdk.StageEvents.STAGE_PARTICIPANT_LEFT, (participant) => {
      setParticipants((prev) => {
        const next = { ...prev };
        delete next[participant.id];
        return next;
      });
    });

    stage.on(sdk.StageEvents.STAGE_PARTICIPANT_STREAMS_ADDED, (participant, streams) => {
      if (participant.isLocal) return;
      setParticipants((prev) => ({ ...prev, [participant.id]: streams }));
    });

    stage.on(sdk.StageEvents.STAGE_PARTICIPANT_STREAMS_REMOVED, (participant) => {
      setParticipants((prev) => ({ ...prev, [participant.id]: [] }));
    });
  }

  async function joinRTStage() {
    const sdk = window.IVSBroadcastClient;
    if (!sdk?.Stage) { setStageError('IVS Broadcast SDK not loaded. Refresh the page.'); return; }
    if (!currentStage?.stage_id) { setStageError('Create a stage first.'); return; }
    if (stageClientRef.current) { setStageError('Already joined a stage. Leave first.'); return; }

    try {
      setStageError('');
      setStatus('Fetching participant token...');
      const result = await requestWithStatus(`/stages/${currentStage.stage_id}/join`, {
        method: 'POST',
        body: JSON.stringify({})
      }, creatorActor);
      if (!result.ok) throw new Error(result.error || `HTTP ${result.status}`);
      const { token: participantToken } = result.data;

      // Ensure local camera is available (reuse existing stream or acquire new one)
      if (!creatorStreamRef.current) {
        creatorStreamRef.current = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
      }
      const localMedia = creatorStreamRef.current;
      applyTrackState(localMedia, {
        videoEnabled: rtCreatorSettings.cameraEnabled,
        audioEnabled: !rtCreatorSettings.muted
      });

      const videoTrack = localMedia.getVideoTracks()[0];
      const audioTrack = localMedia.getAudioTracks()[0];

      // Build local streams for publishing
      const localStreams = [];
      if (videoTrack) localStreams.push(new sdk.LocalStageStream(videoTrack));
      if (audioTrack) localStreams.push(new sdk.LocalStageStream(audioTrack));

      // Strategy: publish local streams; subscribe to everyone else
      const strategy = {
        stageStreamsToPublish() { return localStreams; },
        shouldPublishParticipant() { return true; },
        shouldSubscribeToParticipant(participant) {
          return participant.isLocal ? sdk.SubscribeType.NONE : sdk.SubscribeType.AUDIO_VIDEO;
        }
      };

      const stage = new sdk.Stage(participantToken, strategy);
      attachStageEventHandlers(stage, sdk, setStageConnState, setStageParticipants);

      setStatus('Connecting to stage...');
      await stage.join();
      stageClientRef.current = stage;
      setStatus('Creator connected to stage.');
    } catch (e) {
      setStageError(e.message);
      setStatus(`Join stage failed: ${e.message}`);
    }
  }

  async function joinRTStageAsViewer() {
    const sdk = window.IVSBroadcastClient;
    if (!sdk?.Stage) { setStageError('IVS Broadcast SDK not loaded. Refresh the page.'); return; }
    if (!currentStage?.stage_id) { setStageError('Create a stage first.'); return; }
    if (stageViewerClientRef.current) { setStageError('Viewer is already joined. Leave first.'); return; }

    const viewer = asActor(rtViewerId.trim() || 'guest-rt-1', 'viewer', rtViewerName.trim() || 'Guest');

    try {
      setStageError('');
      setStatus(`Fetching viewer token for ${viewer.user_id}...`);
      const result = await requestWithStatus(`/stages/${currentStage.stage_id}/join`, {
        method: 'POST',
        body: JSON.stringify({})
      }, viewer);
      if (!result.ok) throw new Error(result.error || `HTTP ${result.status}`);
      const { token: participantToken } = result.data;

      let localStreams = [];
      if (currentStage?.mode === 'CALL') {
        const guestReady = await openGuestCameraScreen();
        if (!guestReady) {
          throw new Error('guest_camera_stream_unavailable');
        }
        applyTrackState(guestStreamRef.current, {
          videoEnabled: canGuestPublishVideo,
          audioEnabled: canGuestPublishAudio
        });
        const guestVideoTrack = guestStreamRef.current?.getVideoTracks?.()[0];
        const guestAudioTrack = guestStreamRef.current?.getAudioTracks?.()[0];
        if (guestVideoTrack && canGuestPublishVideo) localStreams.push(new sdk.LocalStageStream(guestVideoTrack));
        if (guestAudioTrack && canGuestPublishAudio) localStreams.push(new sdk.LocalStageStream(guestAudioTrack));
      }

      const strategy = {
        stageStreamsToPublish() { return localStreams; },
        shouldPublishParticipant() { return currentStage?.mode === 'CALL'; },
        shouldSubscribeToParticipant(participant) {
          return participant.isLocal ? sdk.SubscribeType.NONE : sdk.SubscribeType.AUDIO_VIDEO;
        }
      };

      const stage = new sdk.Stage(participantToken, strategy);
      attachStageEventHandlers(stage, sdk, setViewerStageConnState, setViewerStageParticipants);

      setStatus(`Connecting guest ${viewer.user_id}...`);
      await stage.join();
      stageViewerClientRef.current = stage;
      setStatus(`Guest ${viewer.user_id} connected to stage via WebRTC.`);
    } catch (e) {
      setStageError(e.message);
      setStatus(`Guest join failed: ${e.message}`);
    }
  }

  async function leaveRTStage() {
    if (stageClientRef.current) {
      stageClientRef.current.leave();
      stageClientRef.current = null;
    }
    setStageParticipants({});
    setStageConnState('disconnected');
    setStatus('Creator left the stage.');
  }

  async function leaveRTViewerStage() {
    if (stageViewerClientRef.current) {
      stageViewerClientRef.current.leave();
      stageViewerClientRef.current = null;
    }
    setViewerStageParticipants({});
    setViewerStageConnState('disconnected');
    stopGuestCamera();
    setStatus('Guest left the stage.');
  }

  async function endRTStage() {
    if (!currentStage?.stage_id) return;
    await leaveRTStage();
    await leaveRTViewerStage();
    try {
      const result = await requestWithStatus(`/stages/${currentStage.stage_id}`, {
        method: 'DELETE'
      }, creatorActor);
      if (!result.ok && result.status !== 404) throw new Error(result.error || `HTTP ${result.status}`);
      setCurrentStage(null);
      setStatus('Stage ended.');
    } catch (e) {
      setStageError(e.message);
      setStatus(`End stage failed: ${e.message}`);
    }
  }

  async function kickParticipant(pid) {
    if (!currentStage?.stage_id) return;
    try {
      const result = await requestWithStatus(
        `/stages/${currentStage.stage_id}/participants/${pid}`,
        { method: 'DELETE', body: JSON.stringify({ reason: 'host_removed' }) },
        creatorActor
      );
      if (!result.ok) throw new Error(result.error || `HTTP ${result.status}`);
      setStatus(`Participant ${pid} disconnected.`);
    } catch (e) {
      setStageError(e.message);
    }
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
    if (!rtCreatorVideoRef.current) {
      return;
    }

    if (!creatorStreamRef.current) {
      rtCreatorVideoRef.current.srcObject = null;
      return;
    }

    rtCreatorVideoRef.current.srcObject = creatorStreamRef.current;
    rtCreatorVideoRef.current.play().catch(() => {
      // autoplay can be blocked by browser policy; controls are still visible.
    });
  }, [showCreatorCameraScreen, creatorStreamRef.current, currentStage?.stage_id, stageConnState]);

  useEffect(() => {
    if (!rtGuestVideoRef.current) {
      return;
    }

    if (!guestStreamRef.current) {
      rtGuestVideoRef.current.srcObject = null;
      return;
    }

    rtGuestVideoRef.current.srcObject = guestStreamRef.current;
    rtGuestVideoRef.current.play().catch(() => {
      // autoplay can be blocked by browser policy; controls are still visible.
    });
  }, [guestStreamRef.current, currentStage?.stage_id, viewerStageConnState]);

  useEffect(() => {
    applyTrackState(creatorStreamRef.current, {
      videoEnabled: rtCreatorSettings.cameraEnabled,
      audioEnabled: !rtCreatorSettings.muted
    });
  }, [rtCreatorSettings.cameraEnabled, rtCreatorSettings.muted]);

  useEffect(() => {
    applyTrackState(guestStreamRef.current, {
      videoEnabled: canGuestPublishVideo,
      audioEnabled: canGuestPublishAudio
    });
  }, [canGuestPublishVideo, canGuestPublishAudio]);

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
      if (stageClientRef.current) {
        stageClientRef.current.leave();
        stageClientRef.current = null;
      }
      if (stageViewerClientRef.current) {
        stageViewerClientRef.current.leave();
        stageViewerClientRef.current = null;
      }
      void stopIvsBrowserBroadcast();
      stopGuestCamera();
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

  const creatorRemoteCount = Object.keys(stageParticipants).length;
  const guestRemoteCount = Object.keys(viewerStageParticipants).length;
  const streamLibraryCount = creatorStreams.length;
  const selectedStreamName = selectedStream?.title || selectedStream?.stream_id || 'No broadcast selected';
  const selectedStreamType = selectedStream?.stream_type || 'broadcast';
  const ingestReady = Boolean(ingestInfo?.ingest_endpoint && ingestInfo?.stream_key);
  const broadcastMetrics = [
    { label: 'Selected broadcast', value: selectedStreamName, tone: 'accent' },
    { label: 'Broadcast status', value: streamStatus || 'UNKNOWN', tone: streamStatus === 'LIVE' ? 'live' : 'neutral' },
    { label: 'IVS signal', value: ivsIsLive === null ? 'UNKNOWN' : ivsIsLive ? 'LIVE' : 'IDLE', tone: ivsIsLive ? 'live' : 'neutral' },
    { label: 'Realtime peers', value: String(creatorRemoteCount + guestRemoteCount), tone: 'neutral' }
  ];

  return (
    <div className="page">
      <div className="ambientOrb ambientOrbA" />
      <div className="ambientOrb ambientOrbB" />
      <div className="ambientGrid" />

      <header className="masthead panel">
        <div className="mastheadCopy">
          <span className="eyebrow">AllAccess Live Suite</span>
          <h1>Broadcast and Realtime Control Room</h1>
          <p>Operate HLS broadcast, stage-based video calling, viewer simulation, and session controls from one polished studio surface.</p>
        </div>
        <div className="heroMetrics">
          {broadcastMetrics.map((metric) => (
            <article key={metric.label} className={`metricCard ${metric.tone}`}>
              <span>{metric.label}</span>
              <strong>{metric.value}</strong>
            </article>
          ))}
        </div>
      </header>

      <div className="dashboardLayout">
        <aside className="sideColumn">
          <section className="panel railCard">
            <div className="sectionHeading">
              <div>
                <span className="eyebrow">Producer Desk</span>
                <h2>Identity and Channel Setup</h2>
              </div>
            </div>
            <div className="formRow">
              <label>Creator ID</label>
              <input value={creatorId} onChange={(event) => setCreatorId(event.target.value)} placeholder="creator-1" />
            </div>
            <div className="formRow">
              <label>Creator Name</label>
              <input value={creatorName} onChange={(event) => setCreatorName(event.target.value)} placeholder="Creator" />
            </div>
            <div className="buttonStack">
              <button className="primaryAction" onClick={createBroadcastChannel}>Create Broadcast Channel</button>
              <button onClick={loadCreatorStreams}>Load Creator Library</button>
              <button onClick={loadIngestForSelected} disabled={!selectedStream?.stream_id}>Load Ingest Credentials</button>
            </div>
            <div className="railMetaGrid">
              <div>
                <span>Library</span>
                <strong>{streamLibraryCount}</strong>
              </div>
              <div>
                <span>Type</span>
                <strong>{selectedStreamType}</strong>
              </div>
              <div>
                <span>Ingest</span>
                <strong>{ingestReady ? 'READY' : 'WAITING'}</strong>
              </div>
              <div>
                <span>Stage</span>
                <strong>{currentStage?.mode || 'IDLE'}</strong>
              </div>
            </div>
          </section>

          <section className="panel railCard">
            <div className="sectionHeading">
              <div>
                <span className="eyebrow">Channel Library</span>
                <h2>Available Broadcasts</h2>
              </div>
            </div>
            <div className="list cinematicList">
              {creatorStreams.length === 0 ? (
                <div className="emptyStateCard">
                  <strong>No channels loaded</strong>
                  <span>Create or load creator streams to populate this list.</span>
                </div>
              ) : creatorStreams.map((stream) => (
                <button
                  key={stream.stream_id}
                  className={`listItem ${selectedStream?.stream_id === stream.stream_id ? 'selected' : ''}`}
                  onClick={() => setSelectedStream(stream)}
                >
                  <strong>{stream.title || stream.stream_id}</strong>
                  <span>{stream.stream_type || 'broadcast'} · {String(stream.status || 'unknown').toUpperCase()}</span>
                </button>
              ))}
            </div>
          </section>

          <section className="panel railCard">
            <div className="sectionHeading">
              <div>
                <span className="eyebrow">Audience Simulation</span>
                <h2>Viewer and Guest Checks</h2>
              </div>
            </div>
            <div className="formRow">
              <label>Primary Viewer ID</label>
              <input value={singleViewerId} onChange={(event) => setSingleViewerId(event.target.value)} placeholder="viewer-1" />
            </div>
            <div className="formRow">
              <label>Multi-viewer IDs</label>
              <textarea value={multiViewerInput} onChange={(event) => setMultiViewerInput(event.target.value)} rows={4} placeholder="viewer-2, viewer-3, viewer-4" />
            </div>
            <div className="buttonStack">
              <button className="viewerAction" onClick={watchAsOne}>Run Single Viewer Flow</button>
              <button className="viewerAction" onClick={watchAsMany}>Run Multi-Viewer Flow</button>
              <button onClick={getRecording}>Fetch Recording</button>
            </div>
            {recordingUrl && (
              <div className="inlineInfoCard">
                <span>Recording URL</span>
                <strong>{recordingUrl}</strong>
              </div>
            )}
          </section>

          <section className="panel railCard">
            <div className="sectionHeading">
              <div>
                <span className="eyebrow">Session Radar</span>
                <h2>Current Readiness</h2>
              </div>
            </div>
            <div className="checklistList">
              {checklist.map((item) => (
                <div key={item.key} className={`checkItem ${item.done ? 'done' : ''}`}>
                  <strong>{item.done ? 'READY' : 'PENDING'}</strong>
                  <span>{item.label}</span>
                </div>
              ))}
            </div>
            {ingestReady && (
              <div className="inlineInfoCard">
                <span>RTMPS server</span>
                <strong>{toRtmpsServer(ingestInfo?.ingest_endpoint)}</strong>
              </div>
            )}
          </section>
        </aside>

        <main className="primaryColumn">
          <section className="panel featurePanel">
            <div className="sectionHeading">
              <div>
                <span className="eyebrow">Broadcast Studio</span>
                <h2>Creator Uplink and Audience Playback</h2>
              </div>
            </div>
            <p className="status">Creator broadcast view is on the left, audience watch view is on the right.</p>
            <div className="studioGrid">
              <div className="studioCard creatorStudio">
                <div className="cardHeaderRow">
                  <div>
                    <h3>Creator Broadcast View</h3>
                    <p>Local camera, ingest startup, and live control.</p>
                  </div>
                  <span className={`statusPill ${streamStatus === 'LIVE' ? 'live' : 'idle'}`}>{streamStatus || 'IDLE'}</span>
                </div>
                {cameraError && <p className="statusWarn">Camera error: {cameraError}</p>}
                <video ref={creatorVideoRef} className="creatorCameraVideo" autoPlay muted playsInline controls />
                <div className="cameraActions">
                  <button className="primaryAction" onClick={openCreatorCameraScreen}>Enable Camera</button>
                  <button className="creatorAction" onClick={startLiveBroadcast}>Start Live Broadcast</button>
                  <button className="stopAction" onClick={stopLiveBroadcast}>Stop Live Broadcast</button>
                </div>
              </div>

              <div className="studioCard viewerStudio">
                <div className="cardHeaderRow">
                  <div>
                    <h3>Audience Watch View</h3>
                    <p>Monitor playback URL and launch viewer verification.</p>
                  </div>
                  <span className={`statusPill ${watchLiveError ? 'idle' : 'live'}`}>{watchLiveError ? 'ATTN' : 'READY'}</span>
                </div>
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

          <section className="panel featurePanel">
            <div className="sectionHeading">
              <div>
                <span className="eyebrow">Monitoring</span>
                <h2>Stream and IVS Signal Health</h2>
              </div>
            </div>
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
            <div className="checklistActions controlStrip">
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
          </section>

          <section className="panel featurePanel stagePanel">
            <div className="sectionHeading">
              <div>
                <span className="eyebrow">Realtime Stage</span>
                <h2>Video Call Floor</h2>
              </div>
            </div>
            <p className="status">
              <strong>CALL</strong> = 1-to-1 (both publish + subscribe) &nbsp;|&nbsp;
              <strong>BROADCAST</strong> = 1-to-many (host publishes, guests watch only via WebRTC)
            </p>

            {stageError && <p className="statusWarn">Stage error: {stageError}</p>}

            <div className="stageConfigGrid">
              <div className="formRow">
                <label>Mode</label>
                <select value={stageMode} onChange={(e) => setStageMode(e.target.value)}>
                  <option value="CALL">CALL (1-to-1)</option>
                  <option value="BROADCAST">BROADCAST (1-to-many)</option>
                </select>
              </div>
              <div className="formRow">
                <label>Title</label>
                <input
                  value={stageTitle}
                  onChange={(e) => setStageTitle(e.target.value)}
                  placeholder="Stage title"
                />
              </div>
            </div>

            <div className="cameraActions">
              <button className="primaryAction" onClick={createRTStage} disabled={!!currentStage}>
                Create Stage
              </button>
              <button onClick={endRTStage} disabled={!currentStage}>
                End Stage (host)
              </button>
            </div>

            {currentStage && (
              <div className="checklistMeta" style={{ marginTop: 8 }}>
                <span><strong>Stage ID:</strong> {currentStage.stage_id}</span>
                <span><strong>Mode:</strong> {currentStage.mode}</span>
                <span className={`statusPill ${stageConnState === 'connected' || viewerStageConnState === 'connected' ? 'live' : 'idle'}`}>
                  creator: {stageConnState || 'not joined'} / guest: {viewerStageConnState || 'not joined'}
                </span>
              </div>
            )}

            <div className="studioGrid" style={{ marginTop: 16 }}>
          <div className="studioCard creatorStudio">
            <h3>Creator View</h3>
            <p className="status">Publishes camera + mic and subscribes to everyone else.</p>
            <video
              ref={rtCreatorVideoRef}
              className="creatorCameraVideo"
              autoPlay
              muted
              playsInline
              controls
            />
            <div className="cameraActions">
              <button className="primaryAction" onClick={openCreatorCameraScreen}>
                Enable Camera Preview
              </button>
              <button
                className="creatorAction"
                onClick={joinRTStage}
                disabled={!currentStage || !!stageClientRef.current}
              >
                Join as Creator
              </button>
              <button className="stopAction" onClick={leaveRTStage} disabled={!stageClientRef.current}>
                Leave Creator
              </button>
            </div>
            <div className="rtSettingsGrid">
              <label className="inlineCheck"><input type="checkbox" checked={rtCreatorSettings.muteAllGuests} onChange={(event) => setRtCreatorSettings((prev) => ({ ...prev, muteAllGuests: event.target.checked }))} />Mute all guest audio</label>
              <label className="inlineCheck"><input type="checkbox" checked={rtCreatorSettings.disableStageChat} onChange={(event) => setRtCreatorSettings((prev) => ({ ...prev, disableStageChat: event.target.checked }))} />Disable comment / chat</label>
              <label className="inlineCheck"><input type="checkbox" checked={rtCreatorSettings.cameraEnabled} onChange={(event) => setRtCreatorSettings((prev) => ({ ...prev, cameraEnabled: event.target.checked }))} />Creator camera on</label>
              <label className="inlineCheck"><input type="checkbox" checked={!rtCreatorSettings.muted} onChange={(event) => setRtCreatorSettings((prev) => ({ ...prev, muted: !event.target.checked }))} />Creator mic on</label>
              <label className="inlineCheck"><input type="checkbox" checked={rtCreatorSettings.overrideChatLock} onChange={(event) => setRtCreatorSettings((prev) => ({ ...prev, overrideChatLock: event.target.checked }))} />Creator can override chat lock</label>
            </div>
            <div className="checklistMeta" style={{ marginTop: 8 }}>
              <span><strong>Creator ID:</strong> {creatorActor.user_id}</span>
              <span className={`statusPill ${stageConnState === 'connected' ? 'live' : 'idle'}`}>
                {stageConnState || 'not joined'}
              </span>
            </div>

            {Object.keys(stageParticipants).length > 0 && (
              <div style={{ marginTop: 10 }}>
                <h3>Creator Remote Participants ({Object.keys(stageParticipants).length})</h3>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12 }}>
                  {Object.entries(stageParticipants).map(([pid, streams]) => (
                    <div key={pid} style={{ width: 240 }}>
                      <StageParticipantView participantId={pid} streams={streams} muted={rtCreatorSettings.muteAllGuests} />
                      <button
                        style={{ marginTop: 4, width: '100%', fontSize: 12 }}
                        onClick={() => kickParticipant(pid)}
                      >
                        Kick
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {Object.keys(stageParticipants).length === 0 && stageConnState === 'connected' && (
              <p className="status" style={{ marginTop: 12 }}>
                Creator connected. Waiting for participants...
              </p>
            )}

            <div className="rtChatPanel">
              <h3>Creator Comments / Chat</h3>
              <div className="rtChatMessages">
                {rtStageChatMessages.length === 0 ? <p className="status">No messages yet.</p> : rtStageChatMessages.map((message) => (
                  <div key={message.id} className="rtChatMessage">
                    <strong>{message.author}</strong>
                    <span>{message.body}</span>
                  </div>
                ))}
              </div>
              <textarea value={rtCreatorChatInput} onChange={(event) => setRtCreatorChatInput(event.target.value)} placeholder="Creator message" disabled={isCreatorChatDisabled} />
              <button className="creatorAction" onClick={() => { sendRTStageChatMessage(creatorActor.username || creatorActor.user_id, rtCreatorChatInput); setRtCreatorChatInput(''); }} disabled={isCreatorChatDisabled}>Send Creator Comment</button>
            </div>
          </div>

          <div className="studioCard viewerStudio">
            <h3>Guest Watch View (WebRTC)</h3>
            <p className="status">Guest watches over WebRTC and, in CALL mode, can also publish local camera + mic.</p>
            {Object.keys(viewerStageParticipants).length > 0 && (
              <div style={{ marginTop: 10 }}>
                <h3>Guest Remote Participants ({Object.keys(viewerStageParticipants).length})</h3>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12 }}>
                  {Object.entries(viewerStageParticipants).map(([pid, streams]) => (
                    <div key={pid} style={{ width: 240 }}>
                      <StageParticipantView participantId={pid} streams={streams} muted={rtGuestSettings.speakerMuted} />
                    </div>
                  ))}
                </div>
              </div>
            )}

            <div className="formRow">
              <label>Guest ID</label>
              <input
                value={rtViewerId}
                onChange={(e) => setRtViewerId(e.target.value)}
                placeholder="guest-rt-1"
              />
            </div>
            <div className="formRow">
              <label>Guest Name</label>
              <input
                value={rtViewerName}
                onChange={(e) => setRtViewerName(e.target.value)}
                placeholder="Guest"
              />
            </div>
            <div className="cameraActions">
              <button className="primaryAction" onClick={openGuestCameraScreen}>
                Enable Guest Preview
              </button>
              <button
                className="viewerAction"
                onClick={joinRTStageAsViewer}
                disabled={!currentStage || !!stageViewerClientRef.current}
              >
                Join as Guest (Watch)
              </button>
              <button className="stopAction" onClick={leaveRTViewerStage} disabled={!stageViewerClientRef.current}>
                Leave Guest
              </button>
            </div>
            <div className="rtSettingsGrid">
              <label className="inlineCheck"><input type="checkbox" checked={rtGuestSettings.cameraEnabled} onChange={(event) => setRtGuestSettings((prev) => ({ ...prev, cameraEnabled: event.target.checked }))} disabled={currentStage?.mode !== 'CALL'} />Guest camera on</label>
              <label className="inlineCheck"><input type="checkbox" checked={!rtGuestSettings.muted} onChange={(event) => setRtGuestSettings((prev) => ({ ...prev, muted: !event.target.checked }))} disabled={currentStage?.mode !== 'CALL'} />Guest mic on</label>
              <label className="inlineCheck"><input type="checkbox" checked={rtGuestSettings.speakerMuted} onChange={(event) => setRtGuestSettings((prev) => ({ ...prev, speakerMuted: event.target.checked }))} />Mute guest speakers</label>
              <label className="inlineCheck"><input type="checkbox" checked={rtGuestSettings.overrideCreatorSettings} onChange={(event) => setRtGuestSettings((prev) => ({ ...prev, overrideCreatorSettings: event.target.checked }))} />Override creator restrictions</label>
            </div>
            <div className="checklistMeta" style={{ marginTop: 8 }}>
              <span><strong>Guest ID:</strong> {rtViewerId || 'guest-rt-1'}</span>
              <span className={`statusPill ${viewerStageConnState === 'connected' ? 'live' : 'idle'}`}>
                {viewerStageConnState || 'not joined'}
              </span>
              <span>guest mode: {currentStage?.mode === 'CALL' ? 'publish + subscribe' : 'watch only'}</span>
            </div>

           
            {Object.keys(viewerStageParticipants).length === 0 && viewerStageConnState === 'connected' && (
              <p className="status" style={{ marginTop: 12 }}>
                Guest connected. Waiting for host/participants media...
              </p>
            )}

            <div className="rtChatPanel">
              <h3>Guest Comments / Chat</h3>
              <div className="rtChatMessages">
                {rtStageChatMessages.length === 0 ? <p className="status">No messages yet.</p> : rtStageChatMessages.map((message) => (
                  <div key={message.id} className="rtChatMessage">
                    <strong>{message.author}</strong>
                    <span>{message.body}</span>
                  </div>
                ))}
              </div>
              <textarea value={rtGuestChatInput} onChange={(event) => setRtGuestChatInput(event.target.value)} placeholder="Guest message" disabled={isGuestChatDisabled} />
              <button className="viewerAction" onClick={() => { sendRTStageChatMessage(rtViewerName || rtViewerId, rtGuestChatInput); setRtGuestChatInput(''); }} disabled={isGuestChatDisabled}>Send Guest Comment</button>
            </div>
          </div>
        </div>
          </section>
        </main>
      </div>

      <footer className="status">Status: {status}</footer>
    </div>
  );
}
