class TreemapEngine {
  constructor(canvasId, tooltipId) {
    this.canvas = document.getElementById(canvasId);
    this.ctx = this.canvas.getContext('2d');
    this.tooltip = document.getElementById(tooltipId);
    this.nodes = [];
    this.hoveredNode = null;
    this.selectedName = '';
    this.highlightQuery = '';
    this.onSelect = null;

    // Luminous, clear, high-contrast palette
    this.palette = [
      '#2563eb', // vivid royal blue
      '#059669', // vivid emerald green
      '#7c3aed', // vivid violet
      '#d97706', // vivid amber
      '#0284c7', // vivid sky blue
      '#e11d48', // vivid rose
      '#0d9488', // vivid teal
      '#c026d3', // vivid fuchsia
      '#ea580c', // vivid orange
      '#4f46e5', // vivid indigo
      '#16a34a', // vivid forest green
    ];

    this.initEvents();
  }

  initEvents() {
    window.addEventListener('resize', () => this.resizeAndDraw());
    if (window.ResizeObserver && this.canvas.parentElement) {
      new ResizeObserver(() => this.resizeAndDraw()).observe(this.canvas.parentElement);
    }

    this.canvas.addEventListener('mousemove', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;

      const hit = this.hitTest(x, y);
      if (hit !== this.hoveredNode) {
        this.hoveredNode = hit;
        this.draw();
        this.updateTooltip(hit, e.clientX, e.clientY);
      } else if (hit) {
        this.updateTooltip(hit, e.clientX, e.clientY);
      }
    });

    this.canvas.addEventListener('mouseleave', () => {
      this.hoveredNode = null;
      this.tooltip.classList.add('hidden');
      this.draw();
    });

    this.canvas.addEventListener('click', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      const hit = this.hitTest(x, y);
      if (hit && !hit.item.isOther) {
        this.selectedName = hit.item.name;
        this.draw();
        if (typeof this.onSelect === 'function') {
          this.onSelect(hit.item);
        }
      }
    });
  }

  resizeAndDraw() {
    if (!this.canvas || !this.canvas.parentElement) return;
    const rect = this.canvas.parentElement.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    this.canvas.width = rect.width * dpr;
    this.canvas.height = rect.height * dpr;
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    this.displayWidth = rect.width;
    this.displayHeight = rect.height;

    this.computeLayout();
    this.draw();
  }

  setData(packages, totalBytes) {
    this.rawPackages = packages || [];
    this.totalBytes = totalBytes || this.rawPackages.reduce((acc, p) => acc + (p.sizeBytes || 0), 0);

    const threshold = this.totalBytes * 0.002;
    const major = [];
    let otherBytes = 0;
    let otherCount = 0;

    for (const p of this.rawPackages) {
      if (p.sizeBytes >= threshold) {
        major.push({ ...p });
      } else {
        otherBytes += p.sizeBytes;
        otherCount++;
      }
    }

    if (otherCount > 0) {
      major.push({
        name: `(Others: ${otherCount} pkgs)`,
        sizeBytes: otherBytes,
        isOther: true
      });
    }

    this.data = major;
    this.resizeAndDraw();
  }

  computeLayout() {
    if (!this.data || this.data.length === 0 || !this.displayWidth || !this.displayHeight) return;
    this.nodes = [];

    let remaining = [...this.data];
    let x = 0, y = 0, w = this.displayWidth, h = this.displayHeight;

    let remainingTotal = remaining.reduce((sum, item) => sum + item.sizeBytes, 0);
    if (remainingTotal === 0) return;

    for (let i = 0; i < remaining.length; i++) {
      const item = remaining[i];
      const isLast = (i === remaining.length - 1);
      const ratio = isLast ? 1 : Math.min(1, item.sizeBytes / remainingTotal);
      remainingTotal = Math.max(1, remainingTotal - item.sizeBytes);

      let nw, nh;
      if (w > h) {
        nw = isLast ? w : Math.min(w, Math.max(2, Math.round(w * ratio)));
        nh = h;
        this.nodes.push({ item, x, y, w: nw, h: nh });
        x += nw;
        w = Math.max(0, w - nw);
      } else {
        nw = w;
        nh = isLast ? h : Math.min(h, Math.max(2, Math.round(h * ratio)));
        this.nodes.push({ item, x, y, w: nw, h: nh });
        y += nh;
        h = Math.max(0, h - nh);
      }
    }
  }

  draw() {
    if (!this.ctx || !this.displayWidth) return;
    this.ctx.clearRect(0, 0, this.displayWidth, this.displayHeight);

    for (let i = 0; i < this.nodes.length; i++) {
      const node = this.nodes[i];
      const isHovered = (node === this.hoveredNode);
      const isSelected = (this.selectedName && node.item.name === this.selectedName);
      const isMatch = this.highlightQuery && node.item.name.toLowerCase().includes(this.highlightQuery);

      this.ctx.save();

      let baseColor;
      if (node.item.isOther) {
        baseColor = '#475569'; // clean slate 600
      } else {
        baseColor = this.palette[i % this.palette.length];
      }

      this.ctx.fillStyle = baseColor;

      // Keep tiles 100% luminous unless searching
      if (this.highlightQuery && !isMatch) {
        this.ctx.globalAlpha = 0.2;
      } else {
        this.ctx.globalAlpha = 1.0;
      }

      this.ctx.fillRect(node.x, node.y, node.w, node.h);

      // Outer tile borders
      if (isSelected) {
        this.ctx.strokeStyle = '#ffffff';
        this.ctx.lineWidth = 3;
      } else if (isHovered) {
        this.ctx.strokeStyle = '#38bdf8';
        this.ctx.lineWidth = 2.5;
      } else {
        this.ctx.strokeStyle = '#0d1117';
        this.ctx.lineWidth = 1.5;
      }
      this.ctx.strokeRect(node.x, node.y, node.w, node.h);

      // Inner subtle border highlight for crisp separation
      if (node.w > 6 && node.h > 6) {
        this.ctx.strokeStyle = isSelected ? 'rgba(56, 189, 248, 0.8)' : 'rgba(255, 255, 255, 0.18)';
        this.ctx.lineWidth = 1;
        this.ctx.strokeRect(node.x + 1.5, node.y + 1.5, Math.max(0, node.w - 3), Math.max(0, node.h - 3));
      }

      // High-contrast labels
      if (node.w > 46 && node.h > 24) {
        this.ctx.save();
        this.ctx.shadowColor = 'rgba(0, 0, 0, 0.95)';
        this.ctx.shadowBlur = 5;
        this.ctx.fillStyle = '#ffffff';
        this.ctx.font = 'bold 12px -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';
        this.ctx.fillText(node.item.name, node.x + 8, node.y + 19, Math.max(0, node.w - 16));

        // Secondary line: size in readable font
        if (node.h > 42 && node.w > 52) {
          this.ctx.fillStyle = '#ffffff';
          this.ctx.font = 'bold 10px "JetBrains Mono", monospace';
          this.ctx.fillText(this.formatBytes(node.item.sizeBytes), node.x + 8, node.y + 34, Math.max(0, node.w - 16));
        }
        this.ctx.restore();
      }

      this.ctx.restore();
    }
  }

  hitTest(x, y) {
    for (const node of this.nodes) {
      if (x >= node.x && x <= node.x + node.w && y >= node.y && y <= node.y + node.h) {
        return node;
      }
    }
    return null;
  }

  updateTooltip(node, clientX, clientY) {
    if (!node) {
      this.tooltip.classList.add('hidden');
      return;
    }
    const pct = this.totalBytes > 0 ? ((node.item.sizeBytes / this.totalBytes) * 100).toFixed(1) : '0.0';
    const escapedName = this.escapeHTML(node.item.name);
    this.tooltip.innerHTML = `
      <div class="font-bold text-[#e6edf3] text-[13px] mb-1">${escapedName}</div>
      <div class="text-[#8b949e] text-xs">Size: <span class="text-[#d29922] font-semibold">${this.formatBytes(node.item.sizeBytes)}</span> <span class="text-[#6e7681]">(${pct}%)</span></div>
      ${node.item.gzipBytes ? `<div class="text-[#8b949e] text-xs mt-0.5">Gzip: ~${this.formatBytes(node.item.gzipBytes)}</div>` : ''}
      ${node.item.chunks && node.item.chunks.length ? `<div class="text-[#6e7681] text-[10px] mt-1 truncate max-w-xs">${node.item.chunks.join(', ')}</div>` : ''}
    `;
    this.tooltip.style.left = `${clientX + 14}px`;
    this.tooltip.style.top = `${clientY + 14}px`;
    this.tooltip.classList.remove('hidden');
  }

  escapeHTML(str) {
    return (str || '').replace(/[&<>"']/g, m => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    })[m]);
  }

  formatBytes(b) {
    if (!b) return '0 B';
    if (b >= 1048576) return (b / 1048576).toFixed(2) + ' MB';
    if (b >= 1024) return (b / 1024).toFixed(1) + ' KB';
    return b + ' B';
  }

  setSelected(name) {
    this.selectedName = name || '';
    this.draw();
  }

  setHighlight(query) {
    this.highlightQuery = (query || '').toLowerCase().trim();
    this.draw();
  }
}
