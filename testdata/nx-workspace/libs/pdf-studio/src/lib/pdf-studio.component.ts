import { Component, ElementRef, ViewChild, AfterViewInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatChipsModule } from '@angular/material/chips';
import * as pdfjsLib from 'pdfjs-dist';
import { ReportDatasetService } from '@nx-workspace/reports';

@Component({
  selector: 'lib-pdf-studio',
  standalone: true,
  imports: [CommonModule, MatCardModule, MatButtonModule, MatIconModule, MatChipsModule],
  template: `
    <div class="pdf-studio">
      <mat-card class="studio-toolbar-card">
        <div class="studio-toolbar">
          <div class="toolbar-title">
            <h2>PDF Report Inspector & Generator</h2>
            <span class="engine-badge">Engine: PDF.js v{{ pdfjsVersion }} + Material</span>
          </div>
          <div class="toolbar-actions">
            <button mat-stroked-button color="primary" (click)="renderPreview()">
              <mat-icon>refresh</mat-icon> Refresh Canvas
            </button>
            <button mat-flat-button color="warn" (click)="exportDocument()">
              <mat-icon>picture_as_pdf</mat-icon> Export Printable Document
            </button>
          </div>
        </div>
      </mat-card>

      <div class="studio-layout">
        <mat-card class="settings-card">
          <h3>Document Structure</h3>
          <div class="setting-item">
            <label>Report Header</label>
            <input type="text" [value]="docTitle" (input)="updateTitle($event)" />
          </div>
          <div class="setting-item">
            <label>Audit Confidentiality</label>
            <select>
              <option>Strictly Confidential - Internal Only</option>
              <option>Executive Board Briefing</option>
              <option>Public Disclosable</option>
            </select>
          </div>
          <div class="summary-box">
            <p><strong>Dataset Items:</strong> {{ dataset.rows().length }}</p>
            <p><strong>Calculated Total:</strong> \${{ dataset.totalAmount() | number:'1.0-0' }}</p>
            <p><strong>Status:</strong> Ready for PDF rasterization</p>
          </div>
        </mat-card>

        <mat-card class="preview-card">
          <div class="preview-header">
            <span>Visual Document Canvas Preview</span>
            <span class="preview-status">Interactive Canvas</span>
          </div>
          <div class="canvas-viewport">
            <canvas #pdfCanvas width="595" height="700"></canvas>
          </div>
        </mat-card>
      </div>
    </div>
  `,
  styles: [`
    .pdf-studio { display: flex; flex-direction: column; gap: 1.25rem; }
    .studio-toolbar { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 1rem 1.5rem; border-radius: 8px; border: 1px solid #e2e8f0; }
    .toolbar-title h2 { margin: 0 0 0.25rem 0; font-size: 1.25rem; color: #0f172a; }
    .engine-badge { font-size: 0.75rem; font-weight: 600; color: #b91c1c; background: #fee2e2; padding: 0.15rem 0.5rem; border-radius: 4px; }
    .toolbar-actions { display: flex; gap: 0.75rem; }
    .btn { padding: 0.5rem 1rem; border-radius: 6px; font-weight: 600; font-size: 0.875rem; cursor: pointer; border: none; }
    .btn-primary { background: #b91c1c; color: white; }
    .btn-secondary { background: #f1f5f9; color: #334155; border: 1px solid #cbd5e1; }
    .studio-layout { display: grid; grid-template-columns: 300px 1fr; gap: 1.5rem; align-items: start; }
    .settings-card { background: #fff; border-radius: 8px; border: 1px solid #e2e8f0; padding: 1.25rem; display: flex; flex-direction: column; gap: 1rem; }
    .settings-card h3 { margin: 0; font-size: 1rem; color: #1e293b; }
    .setting-item label { display: block; font-size: 0.8rem; font-weight: 600; color: #475569; margin-bottom: 0.25rem; }
    .setting-item input, .setting-item select { width: 100%; padding: 0.4rem 0.6rem; border: 1px solid #cbd5e1; border-radius: 6px; font-size: 0.85rem; box-sizing: border-box; }
    .summary-box { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; padding: 0.75rem; font-size: 0.8rem; color: #334155; }
    .summary-box p { margin: 0.25rem 0; }
    .preview-card { background: #fff; border-radius: 8px; border: 1px solid #e2e8f0; padding: 1.25rem; display: flex; flex-direction: column; gap: 1rem; }
    .preview-header { display: flex; justify-content: space-between; font-size: 0.85rem; font-weight: 600; color: #475569; }
    .preview-status { color: #059669; }
    .canvas-viewport { display: flex; justify-content: center; background: #475569; padding: 1.5rem; border-radius: 6px; overflow: auto; }
    canvas { background: #ffffff; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1); border-radius: 2px; }
  `]
})
export class PdfStudioComponent implements AfterViewInit {
  @ViewChild('pdfCanvas') canvasRef!: ElementRef<HTMLCanvasElement>;
  readonly dataset = inject(ReportDatasetService);
  readonly pdfjsVersion = pdfjsLib.version || '4.x';
  docTitle = 'Executive Financial Audit Report';

  ngAfterViewInit() {
    this.renderPreview();
  }

  updateTitle(event: Event) {
    this.docTitle = (event.target as HTMLInputElement).value;
    this.renderPreview();
  }

  renderPreview() {
    const canvas = this.canvasRef.nativeElement;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    ctx.fillStyle = '#1e293b';
    ctx.font = 'bold 20px -apple-system, sans-serif';
    ctx.fillText('OmniReport Executive Report', 40, 50);

    ctx.fillStyle = '#64748b';
    ctx.font = '13px -apple-system, sans-serif';
    ctx.fillText(this.docTitle, 40, 75);
    ctx.fillText(`Generated: ${new Date().toLocaleDateString()}`, 40, 95);

    ctx.strokeStyle = '#e2e8f0';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(40, 110);
    ctx.lineTo(555, 110);
    ctx.stroke();

    ctx.fillStyle = '#0f172a';
    ctx.font = 'bold 12px -apple-system, sans-serif';
    ctx.fillText('CATEGORY', 40, 135);
    ctx.fillText('LINE ITEM', 200, 135);
    ctx.fillText('AMOUNT', 420, 135);
    ctx.fillText('STATUS', 500, 135);

    let y = 160;
    ctx.font = '12px -apple-system, sans-serif';
    this.dataset.rows().forEach(row => {
      ctx.fillStyle = '#334155';
      ctx.fillText(row.category, 40, y);
      ctx.fillText(row.item, 200, y);
      ctx.fillText(`$${row.amount.toLocaleString()}`, 420, y);
      ctx.fillStyle = row.status === 'Approved' ? '#15803d' : '#a16207';
      ctx.fillText(row.status, 500, y);
      y += 28;
    });

    ctx.strokeStyle = '#e2e8f0';
    ctx.beginPath();
    ctx.moveTo(40, y + 10);
    ctx.lineTo(555, y + 10);
    ctx.stroke();

    ctx.fillStyle = '#0f172a';
    ctx.font = 'bold 13px -apple-system, sans-serif';
    ctx.fillText('Total Summary:', 200, y + 35);
    ctx.fillText(`$${this.dataset.totalAmount().toLocaleString()}`, 420, y + 35);
  }

  exportDocument() {
    const canvas = this.canvasRef.nativeElement;
    const link = document.createElement('a');
    link.download = `OmniReport_Audit_Report_${new Date().toISOString().slice(0,10)}.png`;
    link.href = canvas.toDataURL();
    link.click();
  }
}
