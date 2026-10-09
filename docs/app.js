// Gifkite Interactive Documentation & Playground Logic

document.addEventListener('DOMContentLoaded', () => {
  const safeRun = (name, fn) => {
    try {
      fn();
    } catch (e) {
      console.warn(`[Gifkite] failed initializing ${name}:`, e);
    }
  };

  safeRun('copyButtons', initCopyButtons);
  safeRun('installSwitcher', initInstallSwitcher);
  safeRun('viewfinder', initViewfinderDemo);
  safeRun('loupe', initLoupeDemo);
  safeRun('dithering', initDitheringDemo);
  safeRun('ripples', initRippleSandbox);
  safeRun('annotations', initAnnotationsDemo);
  safeRun('trimTimeline', initTrimTimeline);
  safeRun('tabs', initTabs);
});

// 1. Copy Buttons
function initCopyButtons() {
  document.querySelectorAll('[data-copy]').forEach(btn => {
    btn.addEventListener('click', async () => {
      const text = btn.getAttribute('data-copy');
      try {
        await navigator.clipboard.writeText(text);
        const original = btn.innerHTML;
        btn.innerHTML = '✓ Copied';
        setTimeout(() => { btn.innerHTML = original; }, 1800);
      } catch (e) {
        console.error('Clipboard copy failed:', e);
      }
    });
  });
}

// 2. Install Switcher
function initInstallSwitcher() {
  const tabs = document.querySelectorAll('.install-tab-btn');
  const panes = document.querySelectorAll('.install-pane');

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      panes.forEach(p => p.classList.remove('active'));

      tab.classList.add('active');
      const platform = tab.getAttribute('data-platform');
      const pane = document.getElementById(`pane-${platform}`);
      if (pane) pane.classList.add('active');
    });
  });
}

// 3. Tab Navigation for Interactive Demos
function initTabs() {
  const tabs = document.querySelectorAll('.tab-btn');
  const views = document.querySelectorAll('.demo-view');

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      views.forEach(v => v.classList.remove('active'));

      tab.classList.add('active');
      const targetId = tab.getAttribute('data-target');
      const targetView = document.getElementById(targetId);
      if (targetView) targetView.classList.add('active');

      if (targetId === 'demo-ripples') {
        if (typeof resizeRipple === 'function') resizeRipple();
        if (typeof startRippleLoop === 'function') startRippleLoop();
      } else {
        if (typeof stopRippleLoop === 'function') stopRippleLoop();
      }

      // Allow display:block to resolve geometry before rendering canvas
      setTimeout(() => {
        if (targetId === 'demo-loupe' && typeof renderLoupeBase === 'function') {
          renderLoupeBase();
        } else if (targetId === 'demo-dither' && typeof renderDitherDemo === 'function') {
          renderDitherDemo();
        } else if (targetId === 'demo-annot' && typeof renderAnnotBase === 'function') {
          renderAnnotBase();
        }
      }, 30);
    });
  });
}

// 4. Viewfinder Live Simulator (8 Handles, Aspect Ratios & Window Snapping)
function initViewfinderDemo() {
  const container = document.getElementById('vf-container');
  const box = document.getElementById('vf-box');
  const dimText = document.getElementById('vf-dim');
  const recTime = document.getElementById('vf-rectime');
  const recDot = document.getElementById('vf-recdot');
  const btnToggle = document.getElementById('vf-toggle-rec');
  const btnFinish = document.getElementById('vf-btn-finish');
  const ratioBtns = document.querySelectorAll('.vf-aspect-btn');
  const snapTargets = document.querySelectorAll('.mock-snap-target');

  if (!container || !box) return;

  let isRecording = true;
  let seconds = 3.4;
  let timer = setInterval(() => {
    if (isRecording) {
      seconds += 0.1;
      if (recTime) recTime.textContent = `00:0${seconds.toFixed(1)}s`;
    }
  }, 100);

  if (btnToggle) {
    btnToggle.addEventListener('click', () => {
      isRecording = !isRecording;
      btnToggle.textContent = isRecording ? 'Pause' : 'Resume';
      if (recDot) recDot.style.opacity = isRecording ? '1' : '0.3';
    });
  }

  if (btnFinish) {
    btnFinish.addEventListener('click', () => {
      const orig = btnFinish.textContent;
      btnFinish.textContent = '✓ Copied!';
      btnFinish.style.background = 'rgba(46, 204, 113, 0.3)';
      btnFinish.style.color = '#2ecc71';
      setTimeout(() => {
        btnFinish.textContent = orig;
        btnFinish.style.background = 'rgba(255, 59, 92, 0.2)';
        btnFinish.style.color = 'var(--coral-light)';
      }, 1600);
    });
  }

  function updateDimensions() {
    if (dimText) {
      const w = Math.round(box.offsetWidth * 2);
      const h = Math.round(box.offsetHeight * 2);
      dimText.textContent = `${w} × ${h} px`;
    }
  }

  // Dragging logic (center drag)
  let isDragging = false;
  let dragMode = null; // null or handle name ('nw', 'se', etc.)
  let startX, startY, initLeft, initTop, initW, initH;

  box.addEventListener('pointerdown', (e) => {
    if (e.target.closest('button') || e.target.closest('.vf-aspect-bar')) return;
    const handle = e.target.closest('.h-dot');
    if (handle) {
      dragMode = handle.getAttribute('data-handle');
    } else {
      dragMode = 'move';
    }
    isDragging = true;
    startX = e.clientX;
    startY = e.clientY;
    initLeft = box.offsetLeft;
    initTop = box.offsetTop;
    initW = box.offsetWidth;
    initH = box.offsetHeight;
    box.style.transition = 'none';
    if (box.setPointerCapture) {
      try { box.setPointerCapture(e.pointerId); } catch (_) {}
    }
  });

  window.addEventListener('pointermove', (e) => {
    if (!isDragging) return;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    const containerW = container.clientWidth;
    const containerH = container.clientHeight;

    if (dragMode === 'move') {
      const maxLeft = Math.max(0, containerW - box.offsetWidth);
      const maxTop = Math.max(0, containerH - box.offsetHeight);
      box.style.left = `${Math.max(0, Math.min(maxLeft, initLeft + dx))}px`;
      box.style.top = `${Math.max(0, Math.min(maxTop, initTop + dy))}px`;
    } else if (dragMode) {
      let newL = initLeft;
      let newT = initTop;
      let newW = initW;
      let newH = initH;

      if (dragMode.includes('e')) newW = Math.max(160, Math.min(containerW - initLeft, initW + dx));
      if (dragMode.includes('s')) newH = Math.max(120, Math.min(containerH - initTop, initH + dy));
      if (dragMode.includes('w')) {
        const potentialW = initW - dx;
        if (potentialW >= 160 && initLeft + dx >= 0) {
          newL = initLeft + dx;
          newW = potentialW;
        }
      }
      if (dragMode.includes('n')) {
        const potentialH = initH - dy;
        if (potentialH >= 120 && initTop + dy >= 0) {
          newT = initTop + dy;
          newH = potentialH;
        }
      }

      box.style.left = `${newL}px`;
      box.style.top = `${newT}px`;
      box.style.width = `${newW}px`;
      box.style.height = `${newH}px`;
    }

    updateDimensions();
  });

  window.addEventListener('pointerup', () => {
    isDragging = false;
    dragMode = null;
  });
  window.addEventListener('pointercancel', () => {
    isDragging = false;
    dragMode = null;
  });

  // Aspect ratio presets
  ratioBtns.forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      ratioBtns.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      const ratio = btn.getAttribute('data-ratio');
      if (ratio === 'free') return;

      const [rw, rh] = ratio.split(':').map(Number);
      if (!rw || !rh) return;

      box.style.transition = 'all 0.3s cubic-bezier(0.16, 1, 0.3, 1)';
      const targetRatio = rw / rh;
      let targetW = box.offsetWidth;
      let targetH = Math.round(targetW / targetRatio);

      if (box.offsetTop + targetH > container.clientHeight) {
        targetH = Math.max(120, container.clientHeight - box.offsetTop - 10);
        targetW = Math.round(targetH * targetRatio);
      }

      box.style.width = `${Math.min(container.clientWidth - box.offsetLeft - 10, targetW)}px`;
      box.style.height = `${targetH}px`;
      setTimeout(() => {
        box.style.transition = 'none';
        updateDimensions();
      }, 300);
    });
  });

  // Mock window snapping
  snapTargets.forEach(target => {
    target.addEventListener('click', () => {
      box.style.transition = 'all 0.35s cubic-bezier(0.16, 1, 0.3, 1)';
      box.style.left = `${target.offsetLeft}px`;
      box.style.top = `${target.offsetTop}px`;
      box.style.width = `${target.offsetWidth}px`;
      box.style.height = `${target.offsetHeight}px`;

      // Flash border on snapped window
      target.style.borderColor = 'var(--coral-light)';
      target.style.boxShadow = '0 0 20px var(--coral-glow)';
      setTimeout(() => {
        target.style.borderColor = '';
        target.style.boxShadow = '';
        box.style.transition = 'none';
        updateDimensions();
      }, 350);
    });
  });

  updateDimensions();
}

// 4. Dithering Interactive Simulator
let currentDither = 'bayer';
function initDitheringDemo() {
  const buttons = document.querySelectorAll('.dither-btn');
  buttons.forEach(btn => {
    btn.addEventListener('click', () => {
      buttons.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      currentDither = btn.getAttribute('data-dither');
      renderDitherDemo();
    });
  });
}

function renderDitherDemo() {
  const canvas = document.getElementById('dither-canvas');
  if (!canvas || !canvas.offsetParent) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.width = 600;
  const h = canvas.height = 240;

  // Draw rich test gradient
  const grad = ctx.createLinearGradient(0, 0, w, h);
  grad.addColorStop(0, '#0f172a');
  grad.addColorStop(0.3, '#ff3b5c');
  grad.addColorStop(0.65, '#ff9f43');
  grad.addColorStop(1, '#00d2d3');

  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, w, h);

  const imgData = ctx.getImageData(0, 0, w, h);
  const data = imgData.data;

  // Bayer 4x4 matrix normalized
  const bayer4 = [
    [ 0,  8,  2, 10],
    [12,  4, 14,  6],
    [ 3, 11,  1,  9],
    [15,  7, 13,  5]
  ];

  const badge = document.getElementById('dither-metric');

  if (currentDither === 'bayer') {
    for (let y = 0; y < h; y++) {
      for (let x = 0; x < w; x++) {
        const idx = (y * w + x) * 4;
        const threshold = (bayer4[y % 4][x % 4] / 16 - 0.5) * 48;
        data[idx]     = Math.min(255, Math.max(0, quantizeStep(data[idx] + threshold, 32)));
        data[idx + 1] = Math.min(255, Math.max(0, quantizeStep(data[idx + 1] + threshold, 32)));
        data[idx + 2] = Math.min(255, Math.max(0, quantizeStep(data[idx + 2] + threshold, 32)));
      }
    }
    if (badge) badge.textContent = 'Size: ~180 KB • Smooth Gradients • Smallest File';
  } else if (currentDither === 'floyd') {
    // Simulated error diffusion appearance
    for (let y = 0; y < h; y++) {
      for (let x = 0; x < w; x++) {
        const idx = (y * w + x) * 4;
        const noise = ((Math.random() - 0.5) * 30);
        data[idx]     = quantizeStep(data[idx] + noise, 40);
        data[idx + 1] = quantizeStep(data[idx + 1] + noise, 40);
        data[idx + 2] = quantizeStep(data[idx + 2] + noise, 40);
      }
    }
    if (badge) badge.textContent = 'Size: ~420 KB • Maximum Color Fidelity • Larger File';
  } else {
    // None
    for (let i = 0; i < data.length; i += 4) {
      data[i]     = quantizeStep(data[i], 64);
      data[i + 1] = quantizeStep(data[i + 1], 64);
      data[i + 2] = quantizeStep(data[i + 2], 64);
    }
    if (badge) badge.textContent = 'Size: ~95 KB • Posterized Color Banding';
  }

  ctx.putImageData(imgData, 0, 0);
}

function quantizeStep(val, step) {
  return Math.round(val / step) * step;
}

// 5. Cursor Spotlight & Click Ripple Sandbox
let resizeRipple = null;
let startRippleLoop = null;
let stopRippleLoop = null;

function initRippleSandbox() {
  const canvas = document.getElementById('ripple-canvas');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  canvas.style.touchAction = 'none';

  function resize() {
    canvas.width = canvas.parentElement ? (canvas.parentElement.clientWidth || 320) : 320;
    canvas.height = 320;
    drawFrame();
  }
  resizeRipple = resize;
  resize();
  window.addEventListener('resize', resize);

  let mouse = { x: canvas.width / 2, y: canvas.height / 2, active: false };
  let ripples = [];
  let isLooping = false;
  let rafId = null;

  function spawnRipple(clientX, clientY, isRight = false) {
    const rect = canvas.getBoundingClientRect();
    const x = Math.max(0, Math.min(canvas.width, clientX - rect.left));
    const y = Math.max(0, Math.min(canvas.height, clientY - rect.top));
    mouse.x = x;
    mouse.y = y;
    mouse.active = true;

    ripples.push({
      x, y,
      radius: 4,
      maxRadius: 36,
      opacity: 0.9,
      color: isRight ? '#ff9f43' : '#ff3b5c'
    });

    if (!isLooping) start();
  }

  canvas.addEventListener('pointermove', (e) => {
    const rect = canvas.getBoundingClientRect();
    mouse.x = Math.max(0, Math.min(canvas.width, e.clientX - rect.left));
    mouse.y = Math.max(0, Math.min(canvas.height, e.clientY - rect.top));
    mouse.active = true;
    if (!isLooping) start();
  });

  canvas.addEventListener('pointerdown', (e) => {
    spawnRipple(e.clientX, e.clientY, e.button === 2);
  });

  canvas.addEventListener('pointerup', () => {
    setTimeout(() => {
      if (ripples.length === 0) {
        mouse.active = false;
        drawFrame();
      }
    }, 1200);
  });

  canvas.addEventListener('pointerleave', () => {
    mouse.active = false;
    drawFrame();
  });

  canvas.addEventListener('contextmenu', e => e.preventDefault());

  function drawFrame() {
    if (!canvas.offsetParent) return;
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // Draw grid lines
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.04)';
    ctx.lineWidth = 1;
    for (let x = 0; x < canvas.width; x += 40) {
      ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, canvas.height); ctx.stroke();
    }
    for (let y = 0; y < canvas.height; y += 40) {
      ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(canvas.width, y); ctx.stroke();
    }

    // Render Ripples
    for (let i = ripples.length - 1; i >= 0; i--) {
      const r = ripples[i];
      r.radius += 1.8;
      r.opacity -= 0.035;

      if (r.opacity <= 0) {
        ripples.splice(i, 1);
        continue;
      }

      ctx.save();
      ctx.beginPath();
      ctx.arc(r.x, r.y, r.radius, 0, Math.PI * 2);
      ctx.strokeStyle = r.color;
      ctx.globalAlpha = r.opacity;
      ctx.lineWidth = 2.5;
      ctx.stroke();

      ctx.beginPath();
      ctx.arc(r.x, r.y, r.radius * 0.5, 0, Math.PI * 2);
      ctx.strokeStyle = r.color;
      ctx.globalAlpha = r.opacity * 0.5;
      ctx.lineWidth = 1.5;
      ctx.stroke();
      ctx.restore();
    }

    // Render Spotlight Halo around cursor
    if (mouse.active) {
      ctx.save();
      const haloGrad = ctx.createRadialGradient(mouse.x, mouse.y, 4, mouse.x, mouse.y, 38);
      haloGrad.addColorStop(0, 'rgba(255, 59, 92, 0.45)');
      haloGrad.addColorStop(0.5, 'rgba(255, 159, 67, 0.2)');
      haloGrad.addColorStop(1, 'rgba(255, 59, 92, 0)');

      ctx.fillStyle = haloGrad;
      ctx.beginPath();
      ctx.arc(mouse.x, mouse.y, 38, 0, Math.PI * 2);
      ctx.fill();

      // Simulated macOS cursor arrow
      ctx.fillStyle = '#ffffff';
      ctx.strokeStyle = '#000000';
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.moveTo(mouse.x, mouse.y);
      ctx.lineTo(mouse.x, mouse.y + 16);
      ctx.lineTo(mouse.x + 4.5, mouse.y + 12);
      ctx.lineTo(mouse.x + 10, mouse.y + 11.5);
      ctx.closePath();
      ctx.fill();
      ctx.stroke();
      ctx.restore();
    }
  }

  function loop() {
    if (!isLooping || !canvas.offsetParent) {
      isLooping = false;
      return;
    }
    drawFrame();
    if (ripples.length > 0 || mouse.active) {
      rafId = requestAnimationFrame(loop);
    } else {
      isLooping = false;
    }
  }

  function start() {
    if (isLooping) return;
    isLooping = true;
    rafId = requestAnimationFrame(loop);
  }

  function stop() {
    isLooping = false;
    if (rafId) {
      cancelAnimationFrame(rafId);
      rafId = null;
    }
  }

  startRippleLoop = () => {
    resize();
    start();
  };
  stopRippleLoop = stop;
}

// 6. Trim & Review Timeline Simulator
function initTrimTimeline() {
  const container = document.getElementById('trim-track');
  const leftHandle = document.getElementById('trim-h-left');
  const rightHandle = document.getElementById('trim-h-right');
  const info = document.getElementById('trim-info');
  const playBtn = document.getElementById('trim-play-toggle');
  const speedBtns = document.querySelectorAll('.speed-btn');

  if (!container || !leftHandle || !rightHandle) return;

  let leftPct = 12;
  let rightPct = 85;
  let speed = 1;
  let isPlaying = false;
  let playheadPct = 12;
  let animId = null;

  speedBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      speedBtns.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      speed = parseFloat(btn.getAttribute('data-speed')) || 1;
    });
  });

  if (playBtn) {
    playBtn.addEventListener('click', () => {
      isPlaying = !isPlaying;
      playBtn.textContent = isPlaying ? '⏸ Pause Loop' : '▶ Play Loop';
      playBtn.style.color = isPlaying ? 'var(--coral-light)' : '';
      if (isPlaying) {
        startLoop();
      } else {
        if (animId) cancelAnimationFrame(animId);
      }
    });
  }

  function startLoop() {
    let lastTime = performance.now();
    function tick(now) {
      if (!isPlaying) return;
      const dt = (now - lastTime) / 1000;
      lastTime = now;

      const range = Math.max(5, rightPct - leftPct);
      const step = (100 / (range / 15)) * speed * dt * 0.35;
      playheadPct += step;
      if (playheadPct >= rightPct) playheadPct = leftPct;

      const totalFrames = 120;
      const curF = Math.round((playheadPct / 100) * totalFrames);
      const startF = Math.round((leftPct / 100) * totalFrames);
      const endF = Math.round((rightPct / 100) * totalFrames);
      const count = Math.max(1, endF - startF);
      const dur = (count / 15).toFixed(1);

      if (info) {
        info.textContent = `Frame ${curF} • Range: ${startF} → ${endF} (${count} frames, ${dur}s @ ${speed}x)`;
      }

      animId = requestAnimationFrame(tick);
    }
    animId = requestAnimationFrame(tick);
  }

  function update() {
    leftHandle.style.left = `${leftPct}%`;
    rightHandle.style.left = `${rightPct}%`;

    const leftOverlay = document.getElementById('trim-ov-left');
    const rightOverlay = document.getElementById('trim-ov-right');

    if (leftOverlay) leftOverlay.style.width = `${leftPct}%`;
    if (rightOverlay) {
      rightOverlay.style.left = `${rightPct}%`;
      rightOverlay.style.width = `${100 - rightPct}%`;
    }

    const totalFrames = 120;
    const startF = Math.round((leftPct / 100) * totalFrames);
    const endF = Math.round((rightPct / 100) * totalFrames);
    const count = Math.max(1, endF - startF);
    const dur = (count / 15).toFixed(1);

    if (info) {
      info.textContent = `Frames: ${startF} → ${endF} (${count} frames, ${dur}s loop at 15 FPS)`;
    }
  }

  function handleDrag(handle, isLeft) {
    let dragging = false;
    handle.addEventListener('pointerdown', (e) => {
      dragging = true;
      e.preventDefault();
      try { handle.setPointerCapture(e.pointerId); } catch (_) {}
    });

    window.addEventListener('pointermove', (e) => {
      if (!dragging) return;
      const rect = container.getBoundingClientRect();
      const pct = Math.max(0, Math.min(100, ((e.clientX - rect.left) / rect.width) * 100));
      if (isLeft) {
        leftPct = Math.min(pct, rightPct - 5);
      } else {
        rightPct = Math.max(pct, leftPct + 5);
      }
      playheadPct = leftPct;
      update();
    });

    const stopDrag = () => { dragging = false; };
    window.addEventListener('pointerup', stopDrag);
    window.addEventListener('pointercancel', stopDrag);
  }

  handleDrag(leftHandle, true);
  handleDrag(rightHandle, false);

  update();
}

// 7. 4x Pixel Loupe Magnifier Interactive Simulator
function initLoupeDemo() {
  const container = document.getElementById('loupe-container');
  const baseCanvas = document.getElementById('loupe-base-canvas');
  const lens = document.getElementById('loupe-lens');
  const lensCanvas = document.getElementById('loupe-lens-canvas');
  const coords = document.getElementById('loupe-coords');

  if (!container || !baseCanvas || !lens || !lensCanvas) return;
  container.style.touchAction = 'none';

  renderLoupeBase();
  window.addEventListener('resize', renderLoupeBase);

  lensCanvas.width = 110;
  lensCanvas.height = 110;

  function updateLens(clientX, clientY) {
    const rect = baseCanvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    const x = Math.max(0, Math.min(rect.width, clientX - rect.left));
    const y = Math.max(0, Math.min(rect.height, clientY - rect.top));

    lens.classList.add('active');

    // Float lens slightly above/beside cursor so user's finger/mouse doesn't cover it
    let lensX = x - 55;
    let lensY = y - 125;
    if (lensY < 10) lensY = y + 25;
    lensX = Math.max(10, Math.min(rect.width - 120, lensX));
    lensY = Math.max(10, Math.min(rect.height - 120, lensY));

    lens.style.left = `${lensX}px`;
    lens.style.top = `${lensY}px`;

    // 4x zoom sample from base canvas
    const ctxLens = lensCanvas.getContext('2d');
    if (ctxLens) {
      ctxLens.imageSmoothingEnabled = false;
      ctxLens.clearRect(0, 0, 110, 110);

      const scaleX = baseCanvas.width / rect.width;
      const scaleY = baseCanvas.height / rect.height;
      const bmpX = x * scaleX;
      const bmpY = y * scaleY;
      const srcSize = 28;
      const half = srcSize / 2;

      // Strictly clamp coordinates inside source canvas to prevent WebKit IndexSizeError
      const sx = Math.max(0, Math.min(Math.max(0, baseCanvas.width - srcSize), bmpX - half));
      const sy = Math.max(0, Math.min(Math.max(0, baseCanvas.height - srcSize), bmpY - half));

      try {
        ctxLens.drawImage(
          baseCanvas,
          sx, sy, srcSize, srcSize,
          0, 0, 110, 110
        );
      } catch (_) {}

      // Render subtle pixel grid
      ctxLens.strokeStyle = 'rgba(255, 255, 255, 0.08)';
      ctxLens.lineWidth = 1;
      const step = 110 / srcSize;
      for (let gx = 0; gx < 110; gx += step) {
        ctxLens.beginPath(); ctxLens.moveTo(gx, 0); ctxLens.lineTo(gx, 110); ctxLens.stroke();
      }
      for (let gy = 0; gy < 110; gy += step) {
        ctxLens.beginPath(); ctxLens.moveTo(0, gy); ctxLens.lineTo(110, gy); ctxLens.stroke();
      }
    }

    if (coords) {
      coords.textContent = `X: ${Math.round(x * 2)}, Y: ${Math.round(y * 2)}`;
    }
  }

  container.addEventListener('pointerdown', (e) => {
    updateLens(e.clientX, e.clientY);
  });

  container.addEventListener('pointermove', (e) => {
    updateLens(e.clientX, e.clientY);
  });

  container.addEventListener('pointerleave', () => {
    lens.classList.remove('active');
  });

  // Initial preview lens position
  setTimeout(() => {
    const r = baseCanvas.getBoundingClientRect();
    if (r.width > 0) updateLens(r.left + r.width * 0.45, r.top + r.height * 0.42);
  }, 150);
}

function renderLoupeBase() {
  const canvas = document.getElementById('loupe-base-canvas');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.width = canvas.parentElement ? (canvas.parentElement.clientWidth || 320) : 320;
  const h = canvas.height = 340;
  const isMobile = w < 480;

  // Background
  ctx.fillStyle = '#0a0d14';
  ctx.fillRect(0, 0, w, h);

  // Window title bar
  ctx.fillStyle = '#161b22';
  ctx.fillRect(0, 0, w, 38);
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
  ctx.beginPath(); ctx.moveTo(0, 38); ctx.lineTo(w, 38); ctx.stroke();

  // Window control dots
  const dots = ['#ff5f56', '#ffbd2e', '#27c93f'];
  dots.forEach((col, i) => {
    ctx.beginPath();
    ctx.arc(16 + i * 14, 19, 4.5, 0, Math.PI * 2);
    ctx.fillStyle = col;
    ctx.fill();
  });

  // Window title text
  ctx.fillStyle = '#8b949e';
  ctx.font = isMobile ? '10px "JetBrains Mono", Menlo, monospace' : '12px "JetBrains Mono", Menlo, monospace';
  const titleText = isMobile ? 'sck.go — 60 FPS' : 'recorder_sck.go — ScreenCaptureKit 60 FPS';
  ctx.fillText(titleText, isMobile ? 65 : 80, 23);

  // Line numbers & Code lines
  const lines = [
    { num: '01', code: 'package main', col: '#ff7b72' },
    { num: '02', code: 'import "github.com/gifkite/sck"', col: '#79c0ff' },
    { num: '03', code: '', col: '' },
    { num: '04', code: isMobile ? 'func Start60FPS() (*Stream) {' : 'func StartHardware60FPS(winID uint32) (*Stream, error) {', col: '#d2a8ff' },
    { num: '05', code: isMobile ? '  cfg := sck.Config{ FPS: 60 }' : '    cfg := sck.Config{ FPS: 60, IsolateWindow: true }', col: '#e6edf3' },
    { num: '06', code: isMobile ? '  stream := sck.New(cfg)' : '    stream, err := sck.NewStream(winID, cfg)', col: '#e6edf3' },
    { num: '07', code: isMobile ? '  return stream.Start()' : '    return stream.StartHardwareCapture()', col: '#7ee787' },
    { num: '08', code: '}', col: '#d2a8ff' },
    { num: '09', code: '// Sub-1% CPU • Zero frame drops', col: '#8b949e' },
  ];

  const codeFontSize = isMobile ? '10px' : '12px';
  lines.forEach((l, idx) => {
    const y = 68 + idx * (isMobile ? 22 : 24);
    ctx.fillStyle = '#484f58';
    ctx.font = `${codeFontSize} "JetBrains Mono", Menlo, monospace`;
    ctx.fillText(l.num, isMobile ? 12 : 20, y);

    if (l.code) {
      ctx.fillStyle = l.col || '#e6edf3';
      ctx.fillText(l.code, isMobile ? 38 : 55, y);
    }
  });

  // Pill badge in corner
  const badgeW = isMobile ? 135 : 160;
  const badgeH = 24;
  const badgeX = Math.max(10, w - badgeW - 12);
  const badgeY = h - 34;

  ctx.fillStyle = 'rgba(255, 59, 92, 0.15)';
  ctx.fillRect(badgeX, badgeY, badgeW, badgeH);
  ctx.strokeStyle = 'rgba(255, 59, 92, 0.4)';
  ctx.strokeRect(badgeX, badgeY, badgeW, badgeH);
  ctx.fillStyle = '#ff6584';
  ctx.font = `bold ${isMobile ? '9px' : '11px'} "JetBrains Mono", Menlo, monospace`;
  ctx.fillText('● 60 FPS • SCK ACTIVE', badgeX + 10, badgeY + 16);
}

// 8. Annotations & Privacy Redaction Interactive Demo
let annotItems = [];
let annotUndoStack = [];
let annotRedoStack = [];
let activeAnnotTool = 'arrow';
let activeAnnotColor = '#ff3b5c';

function pushAnnotHistory() {
  annotUndoStack.push(JSON.parse(JSON.stringify(annotItems)));
  if (annotUndoStack.length > 30) annotUndoStack.shift();
  annotRedoStack = [];
}

function initAnnotationsDemo() {
  const stage = document.getElementById('annot-stage');
  const canvas = document.getElementById('annot-canvas');
  const resetBtn = document.getElementById('annot-reset-btn');
  const undoBtn = document.getElementById('annot-undo-btn');
  const redoBtn = document.getElementById('annot-redo-btn');
  const toolBtns = document.querySelectorAll('.annot-tool-btn[data-tool]');
  const colorSwatches = document.querySelectorAll('.color-swatch');

  if (!canvas || !stage) return;
  canvas.style.touchAction = 'none';

  renderAnnotBase();
  window.addEventListener('resize', renderAnnotBase);

  toolBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      toolBtns.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      activeAnnotTool = btn.getAttribute('data-tool');
      applySampleAnnotation(activeAnnotTool);
    });
  });

  colorSwatches.forEach(swatch => {
    swatch.addEventListener('click', () => {
      colorSwatches.forEach(s => s.classList.remove('active'));
      swatch.classList.add('active');
      activeAnnotColor = swatch.getAttribute('data-color') || '#ff3b5c';
      // recolor last arrow/caption if present
      if (annotItems.length > 0) {
        const last = annotItems[annotItems.length - 1];
        if (last.type === 'arrow' || last.type === 'caption') {
          pushAnnotHistory();
          last.color = activeAnnotColor;
          renderAnnotBase();
        }
      }
    });
  });

  if (undoBtn) {
    undoBtn.addEventListener('click', () => {
      if (annotUndoStack.length === 0) return;
      annotRedoStack.push(JSON.parse(JSON.stringify(annotItems)));
      annotItems = annotUndoStack.pop();
      renderAnnotBase();
    });
  }

  if (redoBtn) {
    redoBtn.addEventListener('click', () => {
      if (annotRedoStack.length === 0) return;
      annotUndoStack.push(JSON.parse(JSON.stringify(annotItems)));
      annotItems = annotRedoStack.pop();
      renderAnnotBase();
    });
  }

  if (resetBtn) {
    resetBtn.addEventListener('click', () => {
      if (annotItems.length === 0) return;
      pushAnnotHistory();
      annotItems = [];
      renderAnnotBase();
    });
  }

  function handleAddPoint(clientX, clientY) {
    const rect = canvas.getBoundingClientRect();
    const x = Math.max(0, Math.min(canvas.width, clientX - rect.left));
    const y = Math.max(0, Math.min(canvas.height, clientY - rect.top));
    pushAnnotHistory();
    annotItems.push({ type: activeAnnotTool, x, y, color: activeAnnotColor });
    renderAnnotBase();
  }

  canvas.addEventListener('pointerdown', (e) => {
    handleAddPoint(e.clientX, e.clientY);
  });

  // Default initial annotations
  annotItems = [
    { type: 'blur', x: 220, y: 135, w: 230, h: 26 },
    { type: 'arrow', x1: 520, y1: 80, x2: 440, y2: 195, text: 'Click to Deploy', color: '#ff3b5c' }
  ];
}

function applySampleAnnotation(tool) {
  const canvas = document.getElementById('annot-canvas');
  const w = canvas ? canvas.width : 500;
  pushAnnotHistory();
  if (tool === 'arrow') {
    annotItems.push({ type: 'arrow', x1: w - 80, y1: 75, x2: Math.max(120, w - 160), y2: 185, text: 'Inspect', color: activeAnnotColor });
  } else if (tool === 'blur') {
    annotItems.push({ type: 'blur' });
  } else if (tool === 'caption') {
    annotItems.push({ type: 'caption', x: Math.max(30, (w / 2) - 80), y: 35, text: 'CONFIDENTIAL', color: activeAnnotColor });
  }
  renderAnnotBase();
}

function renderAnnotBase() {
  const canvas = document.getElementById('annot-canvas');
  if (!canvas || !canvas.offsetParent) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.width = canvas.parentElement ? (canvas.parentElement.clientWidth || 320) : 320;
  const h = canvas.height = 300;
  const isMobile = w < 480;

  // Background
  ctx.fillStyle = '#0a0d14';
  ctx.fillRect(0, 0, w, h);

  // App card
  const cardX = isMobile ? 12 : 30;
  const cardY = 20;
  const cardW = w - cardX * 2;
  const cardH = h - 40;

  ctx.fillStyle = '#161b22';
  ctx.roundRect ? ctx.roundRect(cardX, cardY, cardW, cardH, 8) : ctx.fillRect(cardX, cardY, cardW, cardH);
  ctx.fill();
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
  ctx.stroke();

  // Card Header
  ctx.fillStyle = '#f0f3f6';
  ctx.font = `bold ${isMobile ? '12px' : '15px'} -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`;
  ctx.fillText(isMobile ? 'Deploy Credentials' : 'Project Environment & Deploy Keys', cardX + 16, 52);

  // Row 1: Endpoint
  ctx.fillStyle = '#8b949e';
  ctx.font = `${isMobile ? '11px' : '13px'} -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`;
  ctx.fillText(isMobile ? 'API:' : 'API Endpoint:', cardX + 16, 95);
  ctx.fillStyle = '#58a6ff';
  ctx.font = `${isMobile ? '10px' : '13px'} "JetBrains Mono", Menlo, monospace`;
  ctx.fillText(isMobile ? 'api.gifkite.cloud' : 'https://api.gifkite.cloud/v1/stream', isMobile ? cardX + 50 : cardX + 130, 95);

  // Row 2: Private Token (Sensitive target)
  ctx.fillStyle = '#8b949e';
  ctx.font = `${isMobile ? '11px' : '13px'} -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`;
  ctx.fillText(isMobile ? 'Token:' : 'Private Token:', cardX + 16, 142);

  const tokenBoxX = isMobile ? cardX + 65 : cardX + 130;
  const tokenBoxW = Math.max(140, Math.min(260, cardW - (tokenBoxX - cardX) - 16));
  ctx.fillStyle = '#0d1117';
  ctx.fillRect(tokenBoxX, 124, tokenBoxW, 26);
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)';
  ctx.strokeRect(tokenBoxX, 124, tokenBoxW, 26);
  ctx.fillStyle = '#ff7b72';
  ctx.font = `${isMobile ? '10px' : '13px'} "JetBrains Mono", Menlo, monospace`;
  ctx.fillText(isMobile ? 'ghp_98K2js...' : 'ghp_98K2jsa01920JfkL08191', tokenBoxX + 8, 142);

  // Action Button
  const btnW = isMobile ? 110 : 160;
  const btnX = Math.max(cardX + 16, cardX + cardW - btnW - 16);
  const btnY = 185;
  ctx.fillStyle = '#238636';
  ctx.roundRect ? ctx.roundRect(btnX, btnY, btnW, 32, 6) : ctx.fillRect(btnX, btnY, btnW, 32);
  ctx.fill();
  ctx.fillStyle = '#ffffff';
  ctx.font = `bold ${isMobile ? '11px' : '13px'} -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`;
  ctx.fillText(isMobile ? 'Deploy' : 'Deploy to Cluster', btnX + (isMobile ? 32 : 22), btnY + 21);

  // Draw Annotations
  annotItems.forEach(item => {
    const itemColor = item.color || activeAnnotColor || '#ff3b5c';
    if (item.type === 'blur') {
      const bx = isMobile ? tokenBoxX : (item.x || tokenBoxX);
      const by = isMobile ? 124 : (item.y || 124);
      const bw = isMobile ? tokenBoxW : (item.w || tokenBoxW);
      const bh = isMobile ? 26 : (item.h || 26);

      const tileSize = 6;
      const cols = Math.ceil(bw / tileSize);
      const rows = Math.ceil(bh / tileSize);

      for (let r = 0; r < rows; r++) {
        for (let c = 0; c < cols; c++) {
          const shade = ((r + c) % 2 === 0) ? '#1f2937' : '#374151';
          ctx.fillStyle = shade;
          ctx.fillRect(bx + c * tileSize, by + r * tileSize, tileSize, tileSize);
        }
      }
      ctx.strokeStyle = itemColor;
      ctx.strokeRect(bx, by, bw, bh);
    } else if (item.type === 'arrow') {
      const fromX = Math.min(w - 30, Math.max(50, item.x1 !== undefined ? Math.min(item.x1, w - 40) : (w - 60)));
      const fromY = item.y1 || 70;
      const toX = isMobile ? (btnX + btnW / 2) : (item.x2 || (btnX + 60));
      const toY = isMobile ? btnY : (item.y2 || 185);

      ctx.save();
      ctx.strokeStyle = itemColor;
      ctx.fillStyle = itemColor;
      ctx.lineWidth = 2.5;
      ctx.lineCap = 'round';

      ctx.beginPath();
      ctx.moveTo(fromX, fromY);
      ctx.quadraticCurveTo(fromX - 15, toY - 30, toX, toY);
      ctx.stroke();

      const angle = Math.atan2(toY - (toY - 30), toX - (fromX - 15));
      const headLen = 10;
      ctx.beginPath();
      ctx.moveTo(toX, toY);
      ctx.lineTo(toX - headLen * Math.cos(angle - Math.PI / 6), toY - headLen * Math.sin(angle - Math.PI / 6));
      ctx.lineTo(toX - headLen * Math.cos(angle + Math.PI / 6), toY - headLen * Math.sin(angle + Math.PI / 6));
      ctx.closePath();
      ctx.fill();

      ctx.font = `bold ${isMobile ? '10px' : '12px'} "Outfit", sans-serif`;
      ctx.fillText(item.text || 'Action', Math.max(20, fromX - 55), fromY - 6);
      ctx.restore();
    } else if (item.type === 'caption') {
      const capW = isMobile ? 150 : 210;
      const cx = Math.max(cardX + 10, Math.min(w - capW - 20, item.x || (w / 2 - capW / 2)));
      const cy = item.y || 35;
      ctx.fillStyle = '#07090d';
      ctx.roundRect ? ctx.roundRect(cx, cy, capW, 26, 13) : ctx.fillRect(cx, cy, capW, 26);
      ctx.fill();
      ctx.strokeStyle = itemColor;
      ctx.stroke();

      ctx.fillStyle = itemColor;
      ctx.font = `bold ${isMobile ? '9px' : '11px'} "JetBrains Mono", Menlo, monospace`;
      ctx.fillText(isMobile ? 'REDACTED KEY' : (item.text || 'REDACTED KEY'), cx + 16, cy + 17);
    }
  });
}


