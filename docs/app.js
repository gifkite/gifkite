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

// 2. Tab Navigation for Interactive Demos
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

      if (targetId === 'demo-loupe' && typeof renderLoupeBase === 'function') {
        renderLoupeBase();
      } else if (targetId === 'demo-dither' && typeof renderDitherDemo === 'function') {
        renderDitherDemo();
      } else if (targetId === 'demo-annot' && typeof renderAnnotBase === 'function') {
        renderAnnotBase();
      }
    });
  });
}

// 3. Viewfinder Live Simulator
function initViewfinderDemo() {
  const container = document.getElementById('vf-container');
  const box = document.getElementById('vf-box');
  const dimText = document.getElementById('vf-dim');
  const recTime = document.getElementById('vf-rectime');
  const recDot = document.getElementById('vf-recdot');
  const btnToggle = document.getElementById('vf-toggle-rec');

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

  // Dragging logic within viewfinder container with touch/pointer support
  let isDragging = false;
  let startX, startY, initLeft, initTop;

  box.addEventListener('pointerdown', (e) => {
    if (e.target.closest('button')) return;
    isDragging = true;
    startX = e.clientX;
    startY = e.clientY;
    initLeft = box.offsetLeft;
    initTop = box.offsetTop;
    box.style.transition = 'none';
    if (box.setPointerCapture) {
      try { box.setPointerCapture(e.pointerId); } catch (_) {}
    }
  });

  window.addEventListener('pointermove', (e) => {
    if (!isDragging) return;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;

    const maxLeft = container.clientWidth - box.offsetWidth;
    const maxTop = container.clientHeight - box.offsetHeight;

    const newLeft = Math.max(0, Math.min(maxLeft, initLeft + dx));
    const newTop = Math.max(0, Math.min(maxTop, initTop + dy));

    box.style.left = `${newLeft}px`;
    box.style.top = `${newTop}px`;
    box.style.position = 'absolute';

    if (dimText) {
      dimText.textContent = `${Math.round(box.offsetWidth * 2)} × ${Math.round(box.offsetHeight * 2)} px`;
    }
  });

  window.addEventListener('pointerup', () => {
    isDragging = false;
  });
  window.addEventListener('pointercancel', () => {
    isDragging = false;
  });
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

  renderDitherDemo();
}

function renderDitherDemo() {
  const canvas = document.getElementById('dither-canvas');
  if (!canvas) return;
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
function initRippleSandbox() {
  const canvas = document.getElementById('ripple-canvas');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');

  function resize() {
    canvas.width = canvas.parentElement.clientWidth;
    canvas.height = 320;
  }
  resize();
  window.addEventListener('resize', resize);

  let mouse = { x: canvas.width / 2, y: canvas.height / 2, active: false };
  let ripples = [];

  canvas.addEventListener('mousemove', (e) => {
    const rect = canvas.getBoundingClientRect();
    mouse.x = e.clientX - rect.left;
    mouse.y = e.clientY - rect.top;
    mouse.active = true;
  });

  canvas.addEventListener('mouseleave', () => {
    mouse.active = false;
  });

  canvas.addEventListener('mousedown', (e) => {
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    const isRight = e.button === 2;

    ripples.push({
      x, y,
      radius: 4,
      maxRadius: 36,
      opacity: 0.9,
      color: isRight ? '#ff9f43' : '#ff3b5c'
    });
  });

  canvas.addEventListener('contextmenu', e => e.preventDefault());

  function loop() {
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
      // Outer ring
      ctx.beginPath();
      ctx.arc(r.x, r.y, r.radius, 0, Math.PI * 2);
      ctx.strokeStyle = r.color;
      ctx.globalAlpha = r.opacity;
      ctx.lineWidth = 2.5;
      ctx.stroke();

      // Inner faint ring
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

    requestAnimationFrame(loop);
  }

  loop();
}

// 6. Trim & Review Timeline Simulator
function initTrimTimeline() {
  const container = document.getElementById('trim-track');
  const leftHandle = document.getElementById('trim-h-left');
  const rightHandle = document.getElementById('trim-h-right');
  const info = document.getElementById('trim-info');

  if (!container || !leftHandle || !rightHandle) return;

  let leftPct = 12;
  let rightPct = 85;

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
    const count = endF - startF;
    const dur = (count / 15).toFixed(1);

    if (info) {
      info.textContent = `Frames: ${startF} → ${endF} (${count} frames, ${dur}s loop at 15 FPS)`;
    }
  }

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

  renderLoupeBase();
  window.addEventListener('resize', renderLoupeBase);

  lensCanvas.width = 110;
  lensCanvas.height = 110;

  function updateLens(clientX, clientY) {
    const rect = baseCanvas.getBoundingClientRect();
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
      const srcSize = 28;
      ctxLens.drawImage(
        baseCanvas,
        x - srcSize / 2, y - srcSize / 2, srcSize, srcSize,
        0, 0, 110, 110
      );

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
  const w = canvas.width = canvas.parentElement.clientWidth || 600;
  const h = canvas.height = 340;

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
    ctx.arc(20 + i * 16, 19, 5, 0, Math.PI * 2);
    ctx.fillStyle = col;
    ctx.fill();
  });

  // Window title text
  ctx.fillStyle = '#8b949e';
  ctx.font = '12px "JetBrains Mono", Menlo, monospace';
  ctx.fillText('recorder_sck.go — ScreenCaptureKit 60 FPS', 80, 23);

  // Line numbers & Code lines
  const lines = [
    { num: '01', code: 'package main', col: '#ff7b72' },
    { num: '02', code: 'import "github.com/gifkite/sck"', col: '#79c0ff' },
    { num: '03', code: '', col: '' },
    { num: '04', code: 'func StartHardware60FPS(winID uint32) (*Stream, error) {', col: '#d2a8ff' },
    { num: '05', code: '    cfg := sck.Config{ FPS: 60, IsolateWindow: true }', col: '#e6edf3' },
    { num: '06', code: '    stream, err := sck.NewStream(winID, cfg)', col: '#e6edf3' },
    { num: '07', code: '    if err != nil { return nil, err }', col: '#ff7b72' },
    { num: '08', code: '    return stream.StartHardwareCapture()', col: '#7ee787' },
    { num: '09', code: '}', col: '#d2a8ff' },
    { num: '10', code: '// Sub-1% CPU usage • Zero frame drops', col: '#8b949e' },
  ];

  lines.forEach((l, idx) => {
    const y = 68 + idx * 24;
    ctx.fillStyle = '#484f58';
    ctx.font = '12px "JetBrains Mono", Menlo, monospace';
    ctx.fillText(l.num, 20, y);

    if (l.code) {
      ctx.fillStyle = l.col || '#e6edf3';
      ctx.fillText(l.code, 55, y);
    }
  });

  // Pill badge in corner
  ctx.fillStyle = 'rgba(255, 59, 92, 0.15)';
  ctx.fillRect(w - 180, h - 38, 160, 26);
  ctx.strokeStyle = 'rgba(255, 59, 92, 0.4)';
  ctx.strokeRect(w - 180, h - 38, 160, 26);
  ctx.fillStyle = '#ff6584';
  ctx.font = 'bold 11px "JetBrains Mono", Menlo, monospace';
  ctx.fillText('● 60 FPS • SCK ACTIVE', w - 165, h - 21);
}

// 8. Annotations & Privacy Redaction Interactive Demo
let annotItems = [];
let activeAnnotTool = 'arrow';

function initAnnotationsDemo() {
  const stage = document.getElementById('annot-stage');
  const canvas = document.getElementById('annot-canvas');
  const resetBtn = document.getElementById('annot-reset-btn');
  const toolBtns = document.querySelectorAll('.annot-tool-btn[data-tool]');

  if (!canvas || !stage) return;

  renderAnnotBase();
  window.addEventListener('resize', renderAnnotBase);

  toolBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      toolBtns.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      activeAnnotTool = btn.getAttribute('data-tool');
      // Apply immediate sample annotation for that tool
      applySampleAnnotation(activeAnnotTool);
    });
  });

  if (resetBtn) {
    resetBtn.addEventListener('click', () => {
      annotItems = [];
      renderAnnotBase();
    });
  }

  canvas.addEventListener('click', (e) => {
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    annotItems.push({ type: activeAnnotTool, x, y });
    renderAnnotBase();
  });

  // Default initial annotations
  annotItems = [
    { type: 'blur', x: 220, y: 135, w: 230, h: 26 },
    { type: 'arrow', x1: 520, y1: 80, x2: 440, y2: 195, text: 'Click to Deploy' }
  ];
  renderAnnotBase();
}

function applySampleAnnotation(tool) {
  if (tool === 'arrow') {
    annotItems.push({ type: 'arrow', x1: 520, y1: 80, x2: 440, y2: 195, text: 'Inspect Here' });
  } else if (tool === 'blur') {
    annotItems.push({ type: 'blur', x: 220, y: 135, w: 230, h: 26 });
  } else if (tool === 'caption') {
    annotItems.push({ type: 'caption', x: 200, y: 40, text: 'CONFIDENTIAL • API SECRET' });
  }
  renderAnnotBase();
}

function renderAnnotBase() {
  const canvas = document.getElementById('annot-canvas');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.width = canvas.parentElement.clientWidth || 600;
  const h = canvas.height = 300;

  // Background
  ctx.fillStyle = '#0a0d14';
  ctx.fillRect(0, 0, w, h);

  // App card
  ctx.fillStyle = '#161b22';
  ctx.roundRect ? ctx.roundRect(30, 25, w - 60, h - 50, 10) : ctx.fillRect(30, 25, w - 60, h - 50);
  ctx.fill();
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
  ctx.stroke();

  // Card Header
  ctx.fillStyle = '#f0f3f6';
  ctx.font = 'bold 15px -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';
  ctx.fillText('Project Environment & Deploy Keys', 55, 60);

  // Row 1: Endpoint
  ctx.fillStyle = '#8b949e';
  ctx.font = '13px -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';
  ctx.fillText('API Endpoint:', 55, 105);
  ctx.fillStyle = '#58a6ff';
  ctx.font = '13px "JetBrains Mono", Menlo, monospace';
  ctx.fillText('https://api.gifkite.cloud/v1/stream', 180, 105);

  // Row 2: Private Token (Sensitive target)
  ctx.fillStyle = '#8b949e';
  ctx.font = '13px -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';
  ctx.fillText('Private Token:', 55, 150);

  ctx.fillStyle = '#0d1117';
  ctx.fillRect(175, 132, 260, 28);
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)';
  ctx.strokeRect(175, 132, 260, 28);
  ctx.fillStyle = '#ff7b72';
  ctx.font = '13px "JetBrains Mono", Menlo, monospace';
  ctx.fillText('ghp_98K2jsa01920JfkL08191', 185, 151);

  // Action Button
  const btnX = Math.min(w - 220, 360);
  ctx.fillStyle = '#238636';
  ctx.roundRect ? ctx.roundRect(btnX, 195, 160, 36, 6) : ctx.fillRect(btnX, 195, 160, 36);
  ctx.fill();
  ctx.fillStyle = '#ffffff';
  ctx.font = 'bold 13px -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';
  ctx.fillText('Deploy to Cluster', btnX + 22, 218);

  // Draw Annotations
  annotItems.forEach(item => {
    if (item.type === 'blur') {
      const bx = item.x || 175;
      const by = item.y || 132;
      const bw = item.w || 260;
      const bh = item.h || 28;

      // Authentic Mosaic Pixel Blur
      const tileSize = 8;
      const cols = Math.ceil(bw / tileSize);
      const rows = Math.ceil(bh / tileSize);

      for (let r = 0; r < rows; r++) {
        for (let c = 0; c < cols; c++) {
          const shade = ((r + c) % 2 === 0) ? '#1f2937' : '#374151';
          ctx.fillStyle = shade;
          ctx.fillRect(bx + c * tileSize, by + r * tileSize, tileSize, tileSize);
        }
      }
      ctx.strokeStyle = 'rgba(255, 59, 92, 0.6)';
      ctx.strokeRect(bx, by, bw, bh);
    } else if (item.type === 'arrow') {
      const fromX = item.x1 || (w - 120);
      const fromY = item.y1 || 80;
      const toX = item.x2 || (btnX + 80);
      const toY = item.y2 || 190;

      ctx.save();
      ctx.strokeStyle = '#ff3b5c';
      ctx.fillStyle = '#ff3b5c';
      ctx.lineWidth = 3;
      ctx.lineCap = 'round';

      // Curved line
      ctx.beginPath();
      ctx.moveTo(fromX, fromY);
      ctx.quadraticCurveTo(fromX - 20, toY - 40, toX, toY);
      ctx.stroke();

      // Arrow head
      const angle = Math.atan2(toY - (toY - 40), toX - (fromX - 20));
      const headLen = 12;
      ctx.beginPath();
      ctx.moveTo(toX, toY);
      ctx.lineTo(toX - headLen * Math.cos(angle - Math.PI / 6), toY - headLen * Math.sin(angle - Math.PI / 6));
      ctx.lineTo(toX - headLen * Math.cos(angle + Math.PI / 6), toY - headLen * Math.sin(angle + Math.PI / 6));
      ctx.closePath();
      ctx.fill();

      // Label
      ctx.font = 'bold 12px "Outfit", sans-serif';
      ctx.fillText(item.text || 'Action', fromX - 60, fromY - 8);
      ctx.restore();
    } else if (item.type === 'caption') {
      const cx = item.x || (w / 2 - 100);
      const cy = item.y || 40;
      ctx.fillStyle = '#07090d';
      ctx.roundRect ? ctx.roundRect(cx, cy, 210, 28, 14) : ctx.fillRect(cx, cy, 210, 28);
      ctx.fill();
      ctx.strokeStyle = 'rgba(255, 59, 92, 0.5)';
      ctx.stroke();

      ctx.fillStyle = '#ff6584';
      ctx.font = 'bold 11px "JetBrains Mono", Menlo, monospace';
      ctx.fillText(item.text || 'REDACTED KEY', cx + 18, cy + 18);
    }
  });
}

