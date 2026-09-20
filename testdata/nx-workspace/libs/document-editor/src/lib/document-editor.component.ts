import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'lib-document-editor',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="document-studio">
      <div class="studio-toolbar">
        <div class="toolbar-title">
          <h2>Rich Document & Markdown Studio</h2>
          <span class="engine-badge">Engine: Live Parser</span>
        </div>
        <div class="toolbar-actions">
          <button class="btn btn-secondary" (click)="insertSnippet('table')">+ Table</button>
          <button class="btn btn-secondary" (click)="insertSnippet('callout')">+ Callout</button>
          <button class="btn btn-primary" (click)="copyMarkdown()">📋 Copy Source</button>
        </div>
      </div>

      <div class="editor-panes">
        <div class="pane editor-pane">
          <div class="pane-header">
            <span>Source Editor (Markdown)</span>
            <span class="char-count">{{ markdownContent.length }} characters | {{ getWordCount() }} words</span>
          </div>
          <textarea [(ngModel)]="markdownContent" spellcheck="false" placeholder="Write document in Markdown..."></textarea>
        </div>

        <div class="pane preview-pane">
          <div class="pane-header">
            <span>Formatted Live Preview</span>
            <span class="badge-live">Live Sync</span>
          </div>
          <div class="preview-body" [innerHTML]="renderHtml(markdownContent)"></div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .document-studio { display: flex; flex-direction: column; gap: 1.25rem; }
    .studio-toolbar { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 1rem 1.5rem; border-radius: 8px; border: 1px solid #e2e8f0; }
    .toolbar-title h2 { margin: 0 0 0.25rem 0; font-size: 1.25rem; color: #0f172a; }
    .engine-badge { font-size: 0.75rem; font-weight: 600; color: #0284c7; background: #e0f2fe; padding: 0.15rem 0.5rem; border-radius: 4px; }
    .toolbar-actions { display: flex; gap: 0.5rem; }
    .btn { padding: 0.45rem 0.85rem; border-radius: 6px; font-weight: 600; font-size: 0.85rem; cursor: pointer; border: none; }
    .btn-primary { background: #0284c7; color: white; }
    .btn-secondary { background: #f1f5f9; color: #334155; border: 1px solid #cbd5e1; }
    .editor-panes { display: grid; grid-template-columns: 1fr 1fr; gap: 1.25rem; min-height: 520px; }
    .pane { background: #fff; border-radius: 8px; border: 1px solid #e2e8f0; display: flex; flex-direction: column; }
    .pane-header { padding: 0.75rem 1rem; border-bottom: 1px solid #e2e8f0; font-size: 0.85rem; font-weight: 600; color: #475569; display: flex; justify-content: space-between; align-items: center; background: #f8fafc; }
    .char-count { font-size: 0.75rem; color: #94a3b8; font-weight: normal; }
    .badge-live { font-size: 0.7rem; color: #10b981; font-weight: 600; }
    textarea { flex: 1; padding: 1rem; border: none; font-family: 'JetBrains Mono', 'Menlo', 'Courier New', monospace; font-size: 0.9rem; line-height: 1.6; resize: none; outline: none; }
    .preview-body { flex: 1; padding: 1.5rem; overflow: auto; font-size: 0.95rem; line-height: 1.6; color: #1e293b; }
    .preview-body h1 { font-size: 1.5rem; margin-top: 0; color: #0f172a; border-bottom: 1px solid #e2e8f0; padding-bottom: 0.4rem; }
    .preview-body h2 { font-size: 1.25rem; color: #1e293b; margin-top: 1rem; }
    .preview-body blockquote { border-left: 4px solid #0284c7; background: #f0f9ff; margin: 1rem 0; padding: 0.75rem 1rem; border-radius: 0 4px 4px 0; }
  `]
})
export class DocumentEditorComponent {
  markdownContent = `# Executive Financial Report & Strategy

> **Status:** Final Review  
> **Prepared by:** OmniReport Enterprise Intelligence Studio

## 1. Executive Summary
This quarterly report evaluates revenue realization, infrastructure commitments, and operational profit margins across primary product vectors.

### Key Milestones
- Enterprise cloud tier migration achieved **68% margin realization**.
- Security & compliance attestation completed on schedule.
- Professional services backlog delivered on budget.

## 2. Recommendations
1. Accelerate dedicated cluster deployment to reduce overage costs.
2. Automate reconciliation workflows between Spreadsheet Ledger and PDF exports.
`;

  getWordCount(): number {
    return this.markdownContent.trim().split(/\s+/).filter(Boolean).length;
  }

  insertSnippet(type: 'table' | 'callout') {
    if (type === 'callout') {
      this.markdownContent += '\n> [!NOTE]\n> Key takeaway or strategic observation.\n';
    } else {
      this.markdownContent += '\n| Metric | Prior Q | Current Q | Delta |\n| --- | --- | --- | --- |\n| Revenue | $250k | $350k | +40% |\n';
    }
  }

  copyMarkdown() {
    navigator.clipboard.writeText(this.markdownContent);
    alert('Markdown copied to clipboard!');
  }

  renderHtml(md: string): string {
    return md
      .replace(/^# (.*$)/gim, '<h1>$1</h1>')
      .replace(/^## (.*$)/gim, '<h2>$1</h2>')
      .replace(/^### (.*$)/gim, '<h3>$1</h3>')
      .replace(/^\> (.*$)/gim, '<blockquote>$1</blockquote>')
      .replace(/\*\*(.*)\*\*/gim, '<strong>$1</strong>')
      .replace(/\*(.*)\*/gim, '<em>$1</em>')
      .replace(/\n$/gim, '<br />')
      .replace(/\n/g, '<br />');
  }
}
