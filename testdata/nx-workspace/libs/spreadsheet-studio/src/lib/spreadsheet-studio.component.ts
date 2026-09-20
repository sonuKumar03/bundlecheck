import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatTableModule } from '@angular/material/table';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { ReportDatasetService, ReportRow, generateExcelWorkbook } from '@nx-workspace/reports';

@Component({
  selector: 'lib-spreadsheet-studio',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    MatTableModule,
    MatButtonModule,
    MatIconModule,
    MatCardModule,
    MatChipsModule,
    MatFormFieldModule,
    MatInputModule
  ],
  template: `
    <div class="spreadsheet-studio">
      <mat-card class="studio-toolbar-card">
        <div class="studio-toolbar">
          <div class="toolbar-title">
            <h2>Spreadsheet & Financial Ledger</h2>
            <span class="engine-badge">Engine: ExcelJS + Angular Material Table</span>
          </div>
          <div class="toolbar-actions">
            <button mat-stroked-button color="primary" (click)="showNewRowModal = !showNewRowModal">
              <mat-icon>add</mat-icon> Insert Line Item
            </button>
            <button mat-flat-button color="primary" (click)="downloadWorkbook()">
              <mat-icon>download</mat-icon> Download .xlsx Workbook
            </button>
          </div>
        </div>
      </mat-card>

      <mat-card *ngIf="showNewRowModal" class="insert-panel">
        <mat-card-header>
          <mat-card-title>Add Line Item</mat-card-title>
        </mat-card-header>
        <mat-card-content>
          <div class="form-row">
            <mat-form-field appearance="outline">
              <mat-label>Category</mat-label>
              <input matInput placeholder="e.g. Cloud Infra" [(ngModel)]="newRowCategory" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>Line Item</mat-label>
              <input matInput placeholder="e.g. Cluster US-East" [(ngModel)]="newRowItem" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>Amount ($)</mat-label>
              <input matInput type="number" placeholder="50000" [(ngModel)]="newRowAmount" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>Margin (0-1)</mat-label>
              <input matInput type="number" step="0.05" placeholder="0.65" [(ngModel)]="newRowMargin" />
            </mat-form-field>
            <div class="modal-buttons">
              <button mat-flat-button color="primary" (click)="confirmAddRow()">Save Item</button>
              <button mat-button (click)="showNewRowModal = false">Cancel</button>
            </div>
          </div>
        </mat-card-content>
      </mat-card>

      <mat-card class="grid-card">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Category</th>
              <th>Item</th>
              <th class="text-right">Amount</th>
              <th class="text-right">Margin</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let row of dataset.rows()">
              <td><code>#{{ row.id }}</code></td>
              <td>{{ row.category }}</td>
              <td><strong>{{ row.item }}</strong></td>
              <td class="text-right">\${{ row.amount | number:'1.0-0' }}</td>
              <td class="text-right">{{ (row.margin * 100) | number:'1.1-1' }}%</td>
              <td>
                <span class="status-pill" [class.approved]="row.status === 'Approved'" [class.pending]="row.status === 'Pending'" [class.draft]="row.status === 'Draft'">
                  {{ row.status }}
                </span>
              </td>
              <td>
                <button class="btn-icon" (click)="deleteRow(row.id)" title="Delete row">🗑️</button>
              </td>
            </tr>
          </tbody>
          <tfoot>
            <tr>
              <td colspan="3"><strong>Totals & Averages</strong></td>
              <td class="text-right"><strong>\${{ dataset.totalAmount() | number:'1.0-0' }}</strong></td>
              <td class="text-right"><strong>{{ dataset.averageMargin() | number:'1.1-1' }}%</strong></td>
              <td colspan="2"></td>
            </tr>
          </tfoot>
        </table>
      </mat-card>
    </div>
  `,
  styles: [`
    .spreadsheet-studio { display: flex; flex-direction: column; gap: 1.25rem; }
    .studio-toolbar { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 1rem 1.5rem; border-radius: 8px; border: 1px solid #e2e8f0; }
    .toolbar-title h2 { margin: 0 0 0.25rem 0; font-size: 1.25rem; color: #0f172a; }
    .engine-badge { font-size: 0.75rem; font-weight: 600; color: #059669; background: #d1fae5; padding: 0.15rem 0.5rem; border-radius: 4px; }
    .toolbar-actions { display: flex; gap: 0.75rem; }
    .btn { padding: 0.5rem 1rem; border-radius: 6px; font-weight: 600; font-size: 0.875rem; cursor: pointer; border: none; }
    .btn-primary { background: #4f46e5; color: white; }
    .btn-secondary { background: #f1f5f9; color: #334155; border: 1px solid #cbd5e1; }
    .btn-sm { padding: 0.4rem 0.75rem; font-size: 0.8rem; }
    .insert-panel { background: #f8fafc; border: 1px solid #cbd5e1; padding: 1rem 1.5rem; border-radius: 8px; }
    .insert-panel h4 { margin: 0 0 0.75rem 0; color: #1e293b; }
    .form-row { display: flex; gap: 0.5rem; flex-wrap: wrap; }
    .form-row input { padding: 0.45rem 0.75rem; border: 1px solid #cbd5e1; border-radius: 6px; font-size: 0.85rem; }
    .grid-card { background: #fff; border-radius: 8px; border: 1px solid #e2e8f0; overflow-x: auto; }
    .data-table { width: 100%; border-collapse: collapse; font-size: 0.9rem; text-align: left; }
    .data-table th { background: #f8fafc; padding: 0.75rem 1rem; font-weight: 600; color: #475569; border-bottom: 1px solid #e2e8f0; }
    .data-table td { padding: 0.75rem 1rem; border-bottom: 1px solid #f1f5f9; color: #1e293b; }
    .data-table tfoot td { background: #f8fafc; border-top: 2px solid #e2e8f0; font-weight: 700; }
    .text-right { text-align: right; }
    .status-pill { padding: 0.2rem 0.5rem; border-radius: 9999px; font-size: 0.75rem; font-weight: 600; }
    .status-pill.approved { background: #dcfce7; color: #15803d; }
    .status-pill.pending { background: #fef9c3; color: #a16207; }
    .status-pill.draft { background: #f1f5f9; color: #64748b; }
    .btn-icon { background: none; border: none; cursor: pointer; font-size: 1rem; padding: 0.2rem; }
  `]
})
export class SpreadsheetStudioComponent {
  readonly dataset = inject(ReportDatasetService);
  showNewRowModal = false;
  newRowCategory = '';
  newRowItem = '';
  newRowAmount: number | null = null;
  newRowMargin: number | null = null;

  confirmAddRow() {
    if (!this.newRowCategory || !this.newRowItem || !this.newRowAmount) return;
    const newRow: ReportRow = {
      id: String(Date.now()).slice(-4),
      category: this.newRowCategory,
      item: this.newRowItem,
      amount: Number(this.newRowAmount),
      margin: Number(this.newRowMargin || 0.5),
      status: 'Approved'
    };
    this.dataset.addRow(newRow);
    this.newRowCategory = '';
    this.newRowItem = '';
    this.newRowAmount = null;
    this.newRowMargin = null;
    this.showNewRowModal = false;
  }

  deleteRow(id: string) {
    this.dataset.deleteRow(id);
  }

  async downloadWorkbook() {
    const blob = await generateExcelWorkbook('OmniReport Financials', this.dataset.rows());
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `OmniReport_Ledger_${new Date().toISOString().slice(0, 10)}.xlsx`;
    a.click();
    window.URL.revokeObjectURL(url);
  }
}
