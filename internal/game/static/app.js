// Proxy Panic — Interactive Go Reverse Proxy Simulator
// Twodo.design Inspired Editorial Craftsmanship
// Topology Canvas, Web Audio Synth, 5 Guided Missions, 60s Surge Challenge, Request Forensics

(function () {
  "use strict";

  // --- State ---
  const state = {
    xp: 0,
    completedMissions: new Set(),
    activeMission: 1,
    soundEnabled: true,
    strategy: "round-robin",
    backends: [],
    recentTraces: [],
    isArcadeRunning: false,
    arcadeTimer: 60,
    arcadeInterval: null,
    arcadeStats: { handled: 0, errors: 0, totalDuration: 0 },
    packets: [], // Animated particles on canvas
  };

  // --- Sound Synthesizer (Web Audio API) ---
  let audioCtx = null;
  function getAudioCtx() {
    if (!audioCtx) {
      const AudioContext = window.AudioContext || window.webkitAudioContext;
      if (AudioContext) audioCtx = new AudioContext();
    }
    if (audioCtx && audioCtx.state === "suspended") {
      audioCtx.resume();
    }
    return audioCtx;
  }

  const Sound = {
    playLaser() {
      if (!state.soundEnabled) return;
      const ctx = getAudioCtx();
      if (!ctx) return;
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "sine";
      osc.frequency.setValueAtTime(650, ctx.currentTime);
      osc.frequency.exponentialRampToValueAtTime(180, ctx.currentTime + 0.12);
      gain.gain.setValueAtTime(0.18, ctx.currentTime);
      gain.gain.linearRampToValueAtTime(0.005, ctx.currentTime + 0.12);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + 0.12);
    },
    playSuccess() {
      if (!state.soundEnabled) return;
      const ctx = getAudioCtx();
      if (!ctx) return;
      const notes = [523.25, 659.25, 783.99, 1046.5]; // C5, E5, G5, C6
      notes.forEach((freq, idx) => {
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        osc.type = "triangle";
        osc.frequency.value = freq;
        gain.gain.setValueAtTime(0.12, ctx.currentTime + idx * 0.08);
        gain.gain.linearRampToValueAtTime(0.001, ctx.currentTime + idx * 0.08 + 0.28);
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.start(ctx.currentTime + idx * 0.08);
        osc.stop(ctx.currentTime + idx * 0.08 + 0.28);
      });
    },
    playAlert() {
      if (!state.soundEnabled) return;
      const ctx = getAudioCtx();
      if (!ctx) return;
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "sawtooth";
      osc.frequency.setValueAtTime(160, ctx.currentTime);
      osc.frequency.setValueAtTime(130, ctx.currentTime + 0.1);
      gain.gain.setValueAtTime(0.2, ctx.currentTime);
      gain.gain.linearRampToValueAtTime(0.01, ctx.currentTime + 0.22);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + 0.22);
    },
    playClick() {
      if (!state.soundEnabled) return;
      const ctx = getAudioCtx();
      if (!ctx) return;
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "sine";
      osc.frequency.setValueAtTime(540, ctx.currentTime);
      gain.gain.setValueAtTime(0.08, ctx.currentTime);
      gain.gain.linearRampToValueAtTime(0.001, ctx.currentTime + 0.04);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + 0.04);
    }
  };

  // --- Network Canvas Visualization (Warm Editorial Style) ---
  const canvas = document.getElementById("network-canvas");
  const ctx = canvas.getContext("2d");

  // Fixed visual node coordinates
  const nodes = {
    client: { x: 95, y: 155, label: "Client Ingress", icon: "💻", color: "#8f8576" },
    proxy:  { x: 380, y: 155, label: "Go Reverse Proxy", icon: "🔀", color: "#e8be6c" },
    alpha:  { x: 665, y: 65,  label: "Backend Alpha", icon: "⚡", color: "#a8c79a", id: "alpha" },
    beta:   { x: 665, y: 155, label: "Backend Beta",  icon: "⚙️", color: "#e8a97a", id: "beta" },
    gamma:  { x: 665, y: 245, label: "Backend Gamma", icon: "🛡️", color: "#c99ae8", id: "gamma" },
  };

  function spawnPacket(targetBackendID, isSuccess = true) {
    const targetNode = nodes[targetBackendID] || nodes.alpha;
    // Client -> Proxy -> Backend -> Client
    state.packets.push({
      startX: nodes.client.x, startY: nodes.client.y,
      endX: nodes.proxy.x,   endY: nodes.proxy.y,
      x: nodes.client.x,     y: nodes.client.y,
      progress: 0, speed: 0.042,
      phase: 1, // 1: Client->Proxy, 2: Proxy->Backend, 3: Backend->Client
      targetNode: targetNode,
      isSuccess: isSuccess,
      color: isSuccess ? "#e8be6c" : "#e06050"
    });
  }

  function renderCanvas() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    const isDark = document.documentElement.getAttribute("data-theme") !== "light";

    // 1. Draw connection beams
    ctx.lineWidth = 1.5;
    [nodes.alpha, nodes.beta, nodes.gamma].forEach(target => {
      const isDead = isBackendDead(target.id);
      ctx.strokeStyle = isDead
        ? (isDark ? "rgba(224, 96, 80, 0.45)" : "rgba(193, 68, 54, 0.5)")
        : (isDark ? "rgba(232, 190, 108, 0.22)" : "rgba(184, 132, 42, 0.28)");
      ctx.setLineDash([4, 4]);
      ctx.beginPath();
      ctx.moveTo(nodes.proxy.x, nodes.proxy.y);
      ctx.lineTo(target.x, target.y);
      ctx.stroke();
    });

    // Beam Client -> Proxy
    ctx.strokeStyle = isDark ? "rgba(232, 190, 108, 0.35)" : "rgba(184, 132, 42, 0.35)";
    ctx.beginPath();
    ctx.moveTo(nodes.client.x, nodes.client.y);
    ctx.lineTo(nodes.proxy.x, nodes.proxy.y);
    ctx.stroke();
    ctx.setLineDash([]); // Reset dash

    // 2. Draw Nodes
    // Client Node
    drawNode(nodes.client, 28, isDark ? "rgba(143, 133, 118, 0.15)" : "rgba(143, 133, 118, 0.12)", nodes.client.color);
    // Proxy Node (Center Hub)
    drawNode(nodes.proxy, 38, isDark ? "rgba(232, 190, 108, 0.2)" : "rgba(184, 132, 42, 0.18)", nodes.proxy.color, true);
    // Upstream Nodes
    [nodes.alpha, nodes.beta, nodes.gamma].forEach(bNode => {
      const isDead = isBackendDead(bNode.id);
      const color = isDead ? (isDark ? "#e06050" : "#c14436") : bNode.color;
      const bg = isDead
        ? (isDark ? "rgba(224, 96, 80, 0.2)" : "rgba(193, 68, 54, 0.15)")
        : (isDark ? "rgba(168, 199, 154, 0.18)" : "rgba(92, 110, 78, 0.12)");
      drawNode(bNode, 30, bg, color);
    });

    // 3. Update & Draw Flying Packets
    for (let i = state.packets.length - 1; i >= 0; i--) {
      const p = state.packets[i];
      p.progress += p.speed;

      if (p.phase === 1) {
        p.x = p.startX + (p.endX - p.startX) * p.progress;
        p.y = p.startY + (p.endY - p.startY) * p.progress;
        if (p.progress >= 1) {
          p.phase = 2;
          p.progress = 0;
          p.startX = nodes.proxy.x;
          p.startY = nodes.proxy.y;
          p.endX = p.targetNode.x;
          p.endY = p.targetNode.y;
        }
      } else if (p.phase === 2) {
        p.x = p.startX + (p.endX - p.startX) * p.progress;
        p.y = p.startY + (p.endY - p.startY) * p.progress;
        if (p.progress >= 1) {
          p.phase = 3;
          p.progress = 0;
          p.startX = p.targetNode.x;
          p.startY = p.targetNode.y;
          p.endX = nodes.client.x;
          p.endY = nodes.client.y;
        }
      } else if (p.phase === 3) {
        p.x = p.startX + (p.endX - p.startX) * p.progress;
        p.y = p.startY + (p.endY - p.startY) * p.progress;
        if (p.progress >= 1) {
          state.packets.splice(i, 1);
          continue;
        }
      }

      // Draw packet glowing sphere
      ctx.save();
      ctx.beginPath();
      ctx.arc(p.x, p.y, 5, 0, Math.PI * 2);
      ctx.fillStyle = p.color;
      ctx.shadowColor = p.color;
      ctx.shadowBlur = 10;
      ctx.fill();
      ctx.restore();
    }

    requestAnimationFrame(renderCanvas);
  }

  function drawNode(node, radius, bgFill, strokeColor, isPulse = false) {
    const isDark = document.documentElement.getAttribute("data-theme") !== "light";
    ctx.save();
    ctx.beginPath();
    ctx.arc(node.x, node.y, radius, 0, Math.PI * 2);
    ctx.fillStyle = bgFill;
    ctx.strokeStyle = strokeColor;
    ctx.lineWidth = 1.8;
    if (isPulse) {
      ctx.shadowColor = strokeColor;
      ctx.shadowBlur = 14;
    }
    ctx.fill();
    ctx.stroke();

    // Icon
    ctx.font = "16px sans-serif";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText(node.icon, node.x, node.y);

    // Label
    ctx.font = "500 11px 'IBM Plex Mono', monospace";
    ctx.fillStyle = isDark ? "#c2baa8" : "#5a5247";
    ctx.fillText(node.label, node.x, node.y + radius + 15);
    ctx.restore();
  }

  function isBackendDead(id) {
    const b = state.backends.find(item => item.mock_id === id);
    return b ? !b.is_healthy || !b.alive : false;
  }

  // --- API Communications ---
  async function fetchStatus() {
    try {
      const res = await fetch("/game/api/status");
      if (!res.ok) return;
      const data = await res.json();
      state.strategy = data.strategy;
      state.backends = data.backends || [];
      state.recentTraces = data.recent_traces || [];

      updateUI();
    } catch (e) {
      console.warn("Status fetch error", e);
    }
  }

  async function sendProxyRequest(method = "GET", path = "/api/users", headers = {}) {
    Sound.playLaser();
    try {
      const res = await fetch("/game/api/send", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ method, path, headers })
      });
      const trace = await res.json();

      // Find matching mock id
      let targetId = "alpha";
      if (trace.selected_backend.toLowerCase().includes("beta")) targetId = "beta";
      if (trace.selected_backend.toLowerCase().includes("gamma")) targetId = "gamma";

      spawnPacket(targetId, trace.status_code === 200);
      addFeedItem(trace);
      await fetchStatus();

      return trace;
    } catch (err) {
      console.error(err);
      Sound.playAlert();
    }
  }

  async function burstTraffic(count = 20, path = "/api/burst") {
    Sound.playLaser();
    try {
      const res = await fetch("/game/api/burst", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ count, path })
      });
      const data = await res.json();

      // Spawn rapid visual burst packets
      for (let i = 0; i < Math.min(count, 15); i++) {
        setTimeout(() => {
          const targets = ["alpha", "beta", "gamma"];
          spawnPacket(targets[i % 3], true);
        }, i * 35);
      }

      await fetchStatus();
      return data;
    } catch (err) {
      console.error(err);
    }
  }

  async function toggleBackendHealth(backendID, action = "toggle") {
    Sound.playClick();
    try {
      const res = await fetch("/game/api/chaos", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ backend_id: backendID, action })
      });
      const data = await res.json();
      if (!data.is_healthy) Sound.playAlert();
      await fetchStatus();
      return data;
    } catch (err) {
      console.error(err);
    }
  }

  async function setStrategy(strategy) {
    Sound.playClick();
    try {
      const res = await fetch("/game/api/strategy", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ strategy })
      });
      const data = await res.json();
      state.strategy = data.strategy;
      updateUI();
    } catch (err) {
      console.error(err);
    }
  }

  // --- UI Update & Rendering ---
  function updateUI() {
    // Strategy badge
    const stratBadge = document.getElementById("strategy-badge");
    if (stratBadge) {
      stratBadge.textContent = state.strategy === "round-robin" ? "Round-Robin" : "Least-Connections";
    }

    // Sync pill options in sandbox
    document.querySelectorAll(".pill-option").forEach(btn => {
      btn.classList.toggle("active", btn.getAttribute("data-strategy") === state.strategy);
    });

    // Healthy Ratio Badge
    const aliveCount = state.backends.filter(b => b.alive && b.is_healthy !== false).length;
    const ratioBadge = document.getElementById("healthy-ratio-badge");
    if (ratioBadge) {
      ratioBadge.textContent = `${aliveCount}/${state.backends.length} Healthy`;
      ratioBadge.className = aliveCount === state.backends.length ? "tag mono healthy-tag" : "tag mono danger-tag";
    }

    // Render Server Cards in Grid
    const serversGrid = document.getElementById("servers-grid");
    if (serversGrid && state.backends.length > 0) {
      serversGrid.innerHTML = state.backends.map(b => {
        const isOffline = !b.alive || b.is_healthy === false;
        const mockId = b.mock_id || "alpha";
        return `
          <div class="server-node-card ${isOffline ? 'offline' : ''}">
            <div class="server-header">
              <span class="server-name">${b.name}</span>
              <span class="status-badge ${isOffline ? 's502' : 's200'}">
                ${isOffline ? '503 Fail' : '200 OK'}
              </span>
            </div>
            <div class="server-stats">
              <div class="stat-line"><span>Latency:</span><span class="mono">${b.latency_ms || 15}ms</span></div>
              <div class="stat-line"><span>Active Conns:</span><span class="mono">${b.active_conns || 0}</span></div>
              <div class="stat-line"><span>Total Reqs:</span><span class="mono">${b.request_count || 0}</span></div>
            </div>
            <div class="server-actions">
              <button class="btn-chaos-toggle ${isOffline ? 'revive' : 'kill'}" data-backend="${mockId}">
                ${isOffline ? '▲ Revive Node' : '▼ Take Offline'}
              </button>
            </div>
          </div>
        `;
      }).join("");

      // Bind chaos toggle buttons
      serversGrid.querySelectorAll(".btn-chaos-toggle").forEach(btn => {
        btn.onclick = () => {
          const id = btn.getAttribute("data-backend");
          toggleBackendHealth(id, "toggle");
        };
      });
    }

    // Update XP & Rank
    document.getElementById("xp-counter").textContent = state.xp;
    const xpPercent = Math.min(100, (state.xp / 1100) * 100);
    document.getElementById("xp-bar-fill").style.width = `${xpPercent}%`;

    const rankBadge = document.getElementById("player-rank");
    if (rankBadge) {
      if (state.xp >= 1000) rankBadge.textContent = "SRE Maestro 👑";
      else if (state.xp >= 600) rankBadge.textContent = "Senior Route Master";
      else if (state.xp >= 300) rankBadge.textContent = "Network Architect";
      else if (state.xp >= 100) rankBadge.textContent = "Proxy Operator";
      else rankBadge.textContent = "Route Cadet";
    }

    // Update mission steps completed styling
    document.querySelectorAll(".step-btn").forEach(btn => {
      const m = parseInt(btn.getAttribute("data-mission"));
      if (state.completedMissions.has(m)) {
        btn.classList.add("completed");
      }
    });
  }

  function addFeedItem(trace) {
    const feedList = document.getElementById("feed-list");
    if (!feedList) return;

    const placeholder = feedList.querySelector(".empty-state");
    if (placeholder) placeholder.remove();

    const item = document.createElement("div");
    item.className = "feed-item";
    const statusClass = trace.status_code === 200 ? "s200" : "s502";
    item.innerHTML = `
      <div class="feed-item-left">
        <span class="status-badge ${statusClass}">${trace.status_code}</span>
        <span><strong>${trace.method}</strong> ${trace.path}</span>
      </div>
      <div class="feed-item-right" style="color:var(--ink-soft);">
        <span>${trace.selected_backend}</span> •
        <span>${trace.total_duration_ms.toFixed(1)}ms</span>
      </div>
    `;

    item.onclick = () => openTraceDrawer(trace);
    feedList.prepend(item);

    // Keep max 15 items
    if (feedList.children.length > 15) {
      feedList.removeChild(feedList.lastChild);
    }

    const countElem = document.getElementById("feed-count");
    if (countElem) {
      countElem.textContent = `${feedList.children.length} recorded`;
    }
  }

  // --- Detective Trace Drawer ---
  function openTraceDrawer(trace) {
    Sound.playClick();
    const drawer = document.getElementById("trace-drawer");
    const backdrop = document.getElementById("drawer-backdrop");

    document.getElementById("drawer-method").textContent = trace.method;
    document.getElementById("drawer-path").textContent = trace.path;
    document.getElementById("drawer-status").textContent = `${trace.status_code} ${trace.status_code === 200 ? 'OK' : 'Bad Gateway'}`;
    document.getElementById("drawer-backend").textContent = trace.selected_backend;
    document.getElementById("drawer-duration").textContent = `${trace.total_duration_ms.toFixed(1)}ms`;
    document.getElementById("drawer-strategy").textContent = trace.balancer_strategy;

    // Timeline
    const timelineElem = document.getElementById("drawer-timeline");
    timelineElem.innerHTML = (trace.timeline || []).map(step => `
      <div class="timeline-step">
        <div class="timeline-header">
          <span>${step.title}</span>
          <span class="timeline-time mono">${step.time_ms.toFixed(1)}ms</span>
        </div>
        <div class="timeline-desc">${step.desc}</div>
      </div>
    `).join("");

    // Headers Added
    const addedElem = document.getElementById("drawer-headers-added");
    addedElem.innerHTML = Object.entries(trace.headers_added || {}).map(([k, v]) => `
      <li><strong>${k}:</strong> ${v}</li>
    `).join("");

    // Headers Stripped
    const strippedElem = document.getElementById("drawer-headers-stripped");
    strippedElem.innerHTML = (trace.headers_stripped || []).map(h => `
      <li><s>${h}</s> (Hop-by-Hop)</li>
    `).join("");

    // Response Body
    document.getElementById("drawer-json-body").textContent = JSON.stringify(trace.response_body || {}, null, 2);

    drawer.classList.add("open");
    backdrop.classList.add("open");
    drawer.setAttribute("aria-hidden", "false");
  }

  function closeTraceDrawer() {
    const drawer = document.getElementById("trace-drawer");
    const backdrop = document.getElementById("drawer-backdrop");
    drawer.classList.remove("open");
    backdrop.classList.remove("open");
    drawer.setAttribute("aria-hidden", "true");
  }

  // --- Missions Academy Engine ---
  const missions = [
    {
      id: 1,
      badge: "Mission 1 • First Hop",
      title: "First Hop — Single Request Forwarding",
      desc: "Experience how the reverse proxy accepts your HTTP request, injects X-Forwarded-For headers, strips hop-by-hop headers, and routes it to an upstream server.",
      renderInteractive(container) {
        container.innerHTML = `
          <div class="mission-interactive-box">
            <div class="input-row">
              <select id="m1-method" class="clean-select mono">
                <option value="GET">GET</option>
                <option value="POST">POST</option>
              </select>
              <input type="text" id="m1-path" class="clean-input mono" value="/api/users" />
              <button class="btn-solid" id="m1-send-btn">Send Request</button>
            </div>
          </div>
          <div id="m1-result" style="display:none;"></div>
        `;

        document.getElementById("m1-send-btn").onclick = async () => {
          const method = document.getElementById("m1-method").value;
          const path = document.getElementById("m1-path").value;
          const trace = await sendProxyRequest(method, path);

          const resultElem = document.getElementById("m1-result");
          resultElem.style.display = "block";
          resultElem.innerHTML = `
            <div class="alert-box success">
              ✓ Request routed cleanly to <strong>${trace.selected_backend}</strong> in ${trace.total_duration_ms.toFixed(1)}ms.
              Click the request entry in the forensics feed below to inspect headers.
            </div>
          `;

          if (!state.completedMissions.has(1)) {
            state.completedMissions.add(1);
            state.xp += 100;
            Sound.playSuccess();
            updateUI();
          }
        };
      }
    },
    {
      id: 2,
      badge: "Mission 2 • Traffic Stampede",
      title: "Traffic Stampede — Load Balancing Distribution",
      desc: "Send a burst of 20 concurrent requests. Compare Round-Robin (equal distribution) against Least-Connections (routes more requests to faster servers).",
      renderInteractive(container) {
        container.innerHTML = `
          <div class="mission-interactive-box">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom: 0.85rem;">
              <span class="mono" style="font-size:0.82rem;">Policy: <strong id="m2-active-policy">${state.strategy}</strong></span>
              <button class="btn-subtle mono" id="m2-toggle-policy">Switch to ${state.strategy === 'round-robin' ? 'Least-Connections' : 'Round-Robin'}</button>
            </div>
            <button class="play-action-btn full-w" id="m2-burst-btn">
              <span class="play-icon"><svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg></span>
              Burst 20 Requests
            </button>
          </div>
          <div id="m2-result" style="display:none;"></div>
        `;

        document.getElementById("m2-toggle-policy").onclick = async () => {
          const next = state.strategy === "round-robin" ? "least-connections" : "round-robin";
          await setStrategy(next);
          document.getElementById("m2-active-policy").textContent = next;
          document.getElementById("m2-toggle-policy").textContent = `Switch to ${next === 'round-robin' ? 'Least-Connections' : 'Round-Robin'}`;
        };

        document.getElementById("m2-burst-btn").onclick = async () => {
          const res = await burstTraffic(20, "/api/stampede");
          const resultElem = document.getElementById("m2-result");
          resultElem.style.display = "block";

          const distEntries = Object.entries(res.distribution || {});
          resultElem.innerHTML = `
            <div class="alert-box success">
              <div style="font-weight:700; margin-bottom:0.4rem; font-family:'Fraunces',serif;">Traffic Distribution (${res.strategy}):</div>
              ${distEntries.map(([k, count]) => `
                <div style="display:flex; justify-content:space-between; font-family:'IBM Plex Mono',monospace; font-size:0.78rem; margin-bottom:0.2rem;">
                  <span>${k}:</span>
                  <span><strong>${count}</strong> reqs (${((count/20)*100).toFixed(0)}%)</span>
                </div>
              `).join("")}
              <div style="margin-top:0.5rem; font-size:0.75rem; color:var(--ink-soft); font-family:'IBM Plex Mono',monospace;">
                p95 Latency: ${res.p95_latency_ms.toFixed(1)}ms | Success Rate: ${res.success_rate_pct.toFixed(0)}%
              </div>
            </div>
          `;

          if (!state.completedMissions.has(2)) {
            state.completedMissions.add(2);
            state.xp += 200;
            Sound.playSuccess();
            updateUI();
          }
        };
      }
    },
    {
      id: 3,
      badge: "Mission 3 • One Server Down",
      title: "One Server Down — Health Check Failover",
      desc: "Simulate a live outage by taking Server Beta offline. Watch the active health checker detect 3 consecutive failures, eject Beta, and safely reroute 100% of traffic with zero dropped requests!",
      renderInteractive(container) {
        container.innerHTML = `
          <div class="mission-interactive-box">
            <p class="lede-sm" style="margin-bottom:0.85rem;">
              Step 1: Take Server Beta offline.<br>
              Step 2: Send test requests to observe automatic ejection and zero 502s.<br>
              Step 3: Revive Server Beta to verify automatic re-admission.
            </p>
            <div style="display:flex; gap:0.5rem;">
              <button class="btn-solid" id="m3-kill-beta" style="background:var(--alert-red); color:#fff;">Take Beta Offline</button>
              <button class="btn-subtle mono" id="m3-test-traffic">Send 10 Test Reqs</button>
            </div>
          </div>
          <div id="m3-result" style="display:none;"></div>
        `;

        document.getElementById("m3-kill-beta").onclick = async () => {
          await toggleBackendHealth("beta", "kill");
          const resultElem = document.getElementById("m3-result");
          resultElem.style.display = "block";
          resultElem.innerHTML = `
            <div class="alert-box danger">
              ⚠️ Backend Beta taken offline. Active health checker will eject it on next pass. Now click "Send 10 Test Reqs".
            </div>
          `;
        };

        document.getElementById("m3-test-traffic").onclick = async () => {
          const res = await burstTraffic(10, "/api/failover-test");
          const resultElem = document.getElementById("m3-result");
          resultElem.style.display = "block";
          resultElem.innerHTML = `
            <div class="alert-box success">
              ✓ Failover Verified: ${res.success_rate_pct.toFixed(0)}% success rate! All traffic cleanly avoided the offline Beta node.
            </div>
          `;

          if (!state.completedMissions.has(3)) {
            state.completedMissions.add(3);
            state.xp += 300;
            Sound.playSuccess();
            updateUI();
          }
        };
      }
    },
    {
      id: 4,
      badge: "Mission 4 • Header Detective",
      title: "Header Detective — Hop-by-Hop & Forwarding Rules",
      desc: "Inspect real HTTP headers. According to RFC 7230, which headers MUST be stripped by a compliant reverse proxy, and which headers are injected to track client origins?",
      renderInteractive(container) {
        container.innerHTML = `
          <div class="mission-interactive-box">
            <p style="font-weight:600; margin-bottom:0.35rem; font-family:'Fraunces', serif;">Detective Question:</p>
            <p class="lede-sm" style="margin-bottom:0.75rem;">
              Which headers must the Go reverse proxy strip to prevent connection-state confusion upstream?
            </p>
            <div style="display:flex; flex-direction:column; gap:0.45rem; font-family:'IBM Plex Mono', monospace; font-size:0.8rem; margin-bottom:1rem;">
              <label style="display:flex; align-items:center; gap:8px; cursor:pointer;"><input type="checkbox" id="m4-opt1" value="Connection"> Connection</label>
              <label style="display:flex; align-items:center; gap:8px; cursor:pointer;"><input type="checkbox" id="m4-opt2" value="Keep-Alive"> Keep-Alive</label>
              <label style="display:flex; align-items:center; gap:8px; cursor:pointer;"><input type="checkbox" id="m4-opt3" value="User-Agent"> User-Agent</label>
              <label style="display:flex; align-items:center; gap:8px; cursor:pointer;"><input type="checkbox" id="m4-opt4" value="Authorization"> Authorization</label>
            </div>
            <button class="btn-solid" id="m4-submit-btn">Verify Investigation</button>
          </div>
          <div id="m4-result" style="display:none;"></div>
        `;

        document.getElementById("m4-submit-btn").onclick = async () => {
          const answers = [];
          if (document.getElementById("m4-opt1").checked) answers.push("Connection");
          if (document.getElementById("m4-opt2").checked) answers.push("Keep-Alive");
          if (document.getElementById("m4-opt3").checked) answers.push("User-Agent");
          if (document.getElementById("m4-opt4").checked) answers.push("Authorization");

          const res = await fetch("/game/api/quiz/check", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ quiz_id: "m4_stripped_headers", answers })
          });
          const data = await res.json();

          const resultElem = document.getElementById("m4-result");
          resultElem.style.display = "block";
          if (data.correct) {
            resultElem.innerHTML = `<div class="alert-box success">🎉 ${data.explanation}</div>`;
            if (!state.completedMissions.has(4)) {
              state.completedMissions.add(4);
              state.xp += data.xp_awarded;
              Sound.playSuccess();
              updateUI();
            }
          } else {
            resultElem.innerHTML = `<div class="alert-box danger">❌ ${data.explanation}</div>`;
            Sound.playAlert();
          }
        };
      }
    },
    {
      id: 5,
      badge: "Mission 5 • Route Master",
      title: "Route Master — Path-Based Rule Precedence",
      desc: "Establish a dedicated route rule directing all authentication endpoints (/api/auth/*) directly to Backend Alpha, bypassing default load balancing.",
      renderInteractive(container) {
        container.innerHTML = `
          <div class="mission-interactive-box">
            <p class="lede-sm" style="margin-bottom:0.75rem;">
              Click "Apply Route Rule" to bind <code>/api/auth/* &rarr; Backend Alpha</code>, then test with a live request.
            </p>
            <div style="display:flex; gap:0.5rem;">
              <button class="btn-subtle mono" id="m5-rule-btn">1. Apply Route Rule</button>
              <button class="btn-solid" id="m5-test-btn" disabled style="opacity:0.6;">2. Test /api/auth/login</button>
            </div>
          </div>
          <div id="m5-result" style="display:none;"></div>
        `;

        document.getElementById("m5-rule-btn").onclick = async () => {
          await fetch("/game/api/route-rule", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ action: "add", prefix: "/api/auth", backend_name: "alpha" })
          });
          Sound.playClick();
          const testBtn = document.getElementById("m5-test-btn");
          testBtn.disabled = false;
          testBtn.style.opacity = "1";
          const resultElem = document.getElementById("m5-result");
          resultElem.style.display = "block";
          resultElem.innerHTML = `<div class="alert-box success">Rule active: <code>/api/auth/* &rarr; Alpha</code>. Now click Test.</div>`;
        };

        document.getElementById("m5-test-btn").onclick = async () => {
          const trace = await sendProxyRequest("POST", "/api/auth/login");
          const resultElem = document.getElementById("m5-result");
          resultElem.style.display = "block";
          resultElem.innerHTML = `
            <div class="alert-box success">
              👑 Route Precedence Verified! Request directly reached <strong>${trace.selected_backend}</strong>.
            </div>
          `;

          if (!state.completedMissions.has(5)) {
            state.completedMissions.add(5);
            state.xp += 250;
            Sound.playSuccess();
            updateUI();
            showShareModal();
          }
        };
      }
    }
  ];

  function loadMission(id) {
    state.activeMission = id;
    const mission = missions.find(m => m.id === id) || missions[0];

    document.querySelectorAll(".step-btn").forEach(btn => {
      btn.classList.toggle("active", parseInt(btn.getAttribute("data-mission")) === id);
    });

    const card = document.getElementById("mission-card");
    card.innerHTML = `
      <span class="mission-badge mono">${mission.badge}</span>
      <h3>${mission.title}</h3>
      <p class="mission-desc">${mission.desc}</p>
      <div id="mission-action-area"></div>
    `;

    mission.renderInteractive(document.getElementById("mission-action-area"));
  }

  // --- Arcade 60-Second Challenge ---
  function startArcadeChallenge() {
    if (state.isArcadeRunning) return;
    state.isArcadeRunning = true;
    state.arcadeTimer = 60;
    state.arcadeStats = { handled: 0, errors: 0, totalDuration: 0 };

    const startBtn = document.getElementById("btn-start-arcade");
    startBtn.disabled = true;
    startBtn.innerHTML = `
      <span class="play-icon"><svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg></span>
      Surge in progress...
    `;

    const logElem = document.getElementById("arcade-log");
    logElem.innerHTML = "";
    addArcadeLog("⚡ Traffic surge initiated: 100 incoming connections!");

    Sound.playLaser();

    state.arcadeInterval = setInterval(async () => {
      state.arcadeTimer--;
      document.getElementById("arcade-timer").textContent = `${state.arcadeTimer}s`;

      // Chaos events
      if (state.arcadeTimer === 45) {
        addArcadeLog("⚠️ CHAOS EVENT: Backend Beta suffered network partition! Ejected by health check.");
        await toggleBackendHealth("beta", "kill");
      }
      if (state.arcadeTimer === 30) {
        addArcadeLog("⚠️ CHAOS EVENT: Backend Alpha latency spiked to 200ms!");
        await fetch("/game/api/chaos", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ backend_id: "alpha", action: "latency", latency_ms: 200 })
        });
      }
      if (state.arcadeTimer === 15) {
        addArcadeLog("💡 TIP: Switch to Least-Connections to route away from high-latency nodes!");
      }

      // Fire bursts of 5-8 requests
      if (state.arcadeTimer % 3 === 0 && state.arcadeStats.handled < 100) {
        const batch = Math.min(8, 100 - state.arcadeStats.handled);
        const res = await burstTraffic(batch, "/api/arcade-surge");
        state.arcadeStats.handled += batch;
        state.arcadeStats.errors += (res.error_count || 0);
        state.arcadeStats.totalDuration += (res.p50_latency_ms * batch);

        const successRate = ((state.arcadeStats.handled - state.arcadeStats.errors) / state.arcadeStats.handled * 100).toFixed(0);
        const avgLat = (state.arcadeStats.totalDuration / state.arcadeStats.handled).toFixed(0);

        document.getElementById("arcade-req-count").textContent = `${state.arcadeStats.handled} / 100`;
        document.getElementById("arcade-success-rate").textContent = `${successRate}%`;
        document.getElementById("arcade-avg-lat").textContent = `${avgLat}ms`;
      }

      if (state.arcadeTimer <= 0 || state.arcadeStats.handled >= 100) {
        endArcadeChallenge();
      }
    }, 1000);
  }

  function endArcadeChallenge() {
    clearInterval(state.arcadeInterval);
    state.isArcadeRunning = false;

    const startBtn = document.getElementById("btn-start-arcade");
    startBtn.disabled = false;
    startBtn.innerHTML = `
      <span class="play-icon"><svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg></span>
      Replay Surge Challenge
    `;

    const successRate = ((state.arcadeStats.handled - state.arcadeStats.errors) / Math.max(1, state.arcadeStats.handled) * 100).toFixed(0);
    addArcadeLog(`🏁 Challenge Complete! Handled ${state.arcadeStats.handled} requests with ${successRate}% success.`);

    if (parseFloat(successRate) >= 90) {
      Sound.playSuccess();
      state.xp += 300;
      updateUI();
      showShareModal(successRate);
    } else {
      Sound.playAlert();
    }
  }

  function addArcadeLog(msg) {
    const logElem = document.getElementById("arcade-log");
    const line = document.createElement("div");
    line.className = "mono log-line";
    line.textContent = `[${new Date().toLocaleTimeString()}] ${msg}`;
    logElem.appendChild(line);
    logElem.scrollTop = logElem.scrollHeight;
  }

  // --- Shareable LinkedIn Card Modal ---
  function showShareModal(successRate = "99.4") {
    const modal = document.getElementById("share-modal");
    const preview = document.getElementById("share-card-preview");

    const text = `🎮 I survived Proxy Panic!
🛡️ Role: Senior Traffic Controller
⭐ Score: ${state.xp} XP | All 5 Missions Completed
⚡ Routed 100/100 requests successfully with ${successRate}% uptime
⚙️ Powered by a custom Go Reverse Proxy & Load Balancer (Round-Robin & Least-Conn)

Explore the code & try the live simulator:
https://github.com/ShantanuK86/reverse-proxy`;

    preview.textContent = text;
    modal.classList.add("open");
    modal.setAttribute("aria-hidden", "false");

    document.getElementById("btn-copy-share").onclick = () => {
      navigator.clipboard.writeText(text);
      const successText = document.getElementById("copy-success-text");
      successText.style.display = "block";
      setTimeout(() => { successText.style.display = "none"; }, 3000);
    };
  }

  // --- Event Bindings & Init ---
  function init() {
    // Canvas start
    renderCanvas();

    // Theme toggle
    const themeBtn = document.getElementById("theme-toggle-btn");
    const themeIcon = document.getElementById("theme-icon");
    function updateThemeIcon() {
      const cur = document.documentElement.getAttribute("data-theme") || "dark";
      if (themeIcon) themeIcon.textContent = cur === "dark" ? "◐" : "◑";
    }
    updateThemeIcon();

    if (themeBtn) {
      themeBtn.onclick = () => {
        Sound.playClick();
        const cur = document.documentElement.getAttribute("data-theme") || "dark";
        const next = cur === "dark" ? "light" : "dark";
        document.documentElement.setAttribute("data-theme", next);
        try { localStorage.setItem("proxy-panic-theme", next); } catch(e){}
        updateThemeIcon();
      };
    }

    // Load initial mission
    loadMission(1);

    // Initial status poll
    fetchStatus();
    setInterval(fetchStatus, 3000);

    // Mode tab buttons (Missions, 60s Panic, Sandbox)
    document.querySelectorAll(".mode-tab").forEach(btn => {
      btn.onclick = () => {
        Sound.playClick();
        document.querySelectorAll(".mode-tab").forEach(b => b.classList.remove("active"));
        document.querySelectorAll(".tab-panel").forEach(p => p.classList.remove("active"));
        btn.classList.add("active");
        const tabID = `tab-${btn.getAttribute("data-tab")}`;
        const target = document.getElementById(tabID);
        if (target) target.classList.add("active");
      };
    });

    // Mission step buttons
    document.querySelectorAll(".step-btn").forEach(btn => {
      btn.onclick = () => {
        Sound.playClick();
        loadMission(parseInt(btn.getAttribute("data-mission")));
      };
    });

    // Sound toggle
    document.getElementById("audio-toggle-btn").onclick = () => {
      state.soundEnabled = !state.soundEnabled;
      document.getElementById("sound-icon").textContent = state.soundEnabled ? "🔊" : "🔇";
    };

    // Reset button
    document.getElementById("btn-reset-all").onclick = async () => {
      Sound.playClick();
      await fetch("/game/api/reset", { method: "POST" });
      await fetchStatus();
    };

    // Arcade start
    document.getElementById("btn-start-arcade").onclick = startArcadeChallenge;

    // Sandbox Controls: Balancing Strategy Pills
    document.querySelectorAll(".pill-option").forEach(btn => {
      btn.onclick = async () => {
        document.querySelectorAll(".pill-option").forEach(b => b.classList.remove("active"));
        btn.classList.add("active");
        const strat = btn.getAttribute("data-strategy");
        await setStrategy(strat);
      };
    });

    document.getElementById("btn-sandbox-send").onclick = () => {
      const method = document.getElementById("sandbox-method").value;
      const path = document.getElementById("sandbox-path").value;
      sendProxyRequest(method, path);
    };

    document.getElementById("btn-burst-10").onclick = () => burstTraffic(10);
    document.getElementById("btn-burst-25").onclick = () => burstTraffic(25);
    document.getElementById("btn-burst-50").onclick = () => burstTraffic(50);

    // Drawer close
    document.getElementById("drawer-close-btn").onclick = closeTraceDrawer;
    document.getElementById("drawer-backdrop").onclick = closeTraceDrawer;

    // Modal close
    document.getElementById("modal-close-btn").onclick = () => {
      document.getElementById("share-modal").classList.remove("open");
    };
  }

  window.addEventListener("DOMContentLoaded", init);
})();
