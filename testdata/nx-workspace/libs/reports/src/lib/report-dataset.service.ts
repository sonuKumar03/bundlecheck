import { Injectable, signal, computed } from '@angular/core';

export interface ReportRow {
  id: string;
  category: string;
  item: string;
  amount: number;
  margin: number;
  status: 'Approved' | 'Pending' | 'Draft';
}

const INITIAL_ROWS: ReportRow[] = [
  { id: '1', category: 'Enterprise Licenses', item: 'Global Cloud Tier 1', amount: 145000, margin: 0.68, status: 'Approved' },
  { id: '2', category: 'Professional Services', item: 'Architecture Audit', amount: 38000, margin: 0.45, status: 'Approved' },
  { id: '3', category: 'Infrastructure', item: 'Dedicated Cluster US-East', amount: 72000, margin: 0.32, status: 'Approved' },
  { id: '4', category: 'Security & Compliance', item: 'SOC-2 Attestation Pack', amount: 24500, margin: 0.85, status: 'Pending' },
  { id: '5', category: 'Support Subscriptions', item: '24/7 Platinum SLA', amount: 56000, margin: 0.74, status: 'Approved' },
  { id: '6', category: 'Data Integration', item: 'Kafka Stream Connector', amount: 19500, margin: 0.58, status: 'Draft' }
];

@Injectable({
  providedIn: 'root'
})
export class ReportDatasetService {
  private readonly _rows = signal<ReportRow[]>(INITIAL_ROWS);
  readonly rows = this._rows.asReadonly();

  readonly totalAmount = computed(() =>
    this._rows().reduce((sum, r) => sum + r.amount, 0)
  );

  readonly averageMargin = computed(() => {
    const r = this._rows();
    if (r.length === 0) return 0;
    return (r.reduce((sum, item) => sum + item.margin, 0) / r.length) * 100;
  });

  readonly approvedCount = computed(() =>
    this._rows().filter(r => r.status === 'Approved').length
  );

  addRow(row: ReportRow) {
    this._rows.update(rows => [...rows, row]);
  }

  updateRow(id: string, patch: Partial<ReportRow>) {
    this._rows.update(rows =>
      rows.map(r => (r.id === id ? { ...r, ...patch } : r))
    );
  }

  deleteRow(id: string) {
    this._rows.update(rows => rows.filter(r => r.id !== id));
  }
}
