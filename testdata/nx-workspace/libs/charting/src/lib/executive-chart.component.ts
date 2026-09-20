import { Component, ElementRef, ViewChild, AfterViewInit, OnDestroy, inject, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatIconModule } from '@angular/material/icon';
import { Chart, registerables } from 'chart.js';
import moment from 'moment';
import { ReportDatasetService } from '@nx-workspace/reports';

Chart.register(...registerables);

@Component({
  selector: 'lib-executive-chart',
  standalone: true,
  imports: [CommonModule, MatCardModule, MatChipsModule, MatIconModule],
  template: `
    <div class="executive-hub">
      <div class="kpi-grid">
        <mat-card class="kpi-card">
          <mat-card-header>
            <mat-icon mat-card-avatar color="primary">payments</mat-icon>
            <mat-card-subtitle class="kpi-label">TOTAL REVENUE</mat-card-subtitle>
            <mat-card-title class="kpi-value">\${{ dataset.totalAmount() | number:'1.0-0' }}</mat-card-title>
          </mat-card-header>
          <mat-card-content>
            <span class="kpi-sub">Updated {{ lastUpdated }}</span>
          </mat-card-content>
        </mat-card>

        <mat-card class="kpi-card">
          <mat-card-header>
            <mat-icon mat-card-avatar color="accent">trending_up</mat-icon>
            <mat-card-subtitle class="kpi-label">AVG PROFIT MARGIN</mat-card-subtitle>
            <mat-card-title class="kpi-value">{{ dataset.averageMargin() | number:'1.1-1' }}%</mat-card-title>
          </mat-card-header>
          <mat-card-content>
            <span class="kpi-sub highlight">Target: 50%</span>
          </mat-card-content>
        </mat-card>

        <mat-card class="kpi-card">
          <mat-card-header>
            <mat-icon mat-card-avatar color="primary">verified</mat-icon>
            <mat-card-subtitle class="kpi-label">APPROVED LINE ITEMS</mat-card-subtitle>
            <mat-card-title class="kpi-value">{{ dataset.approvedCount() }} / {{ dataset.rows().length }}</mat-card-title>
          </mat-card-header>
          <mat-card-content>
            <span class="kpi-sub">Ready for Audit</span>
          </mat-card-content>
        </mat-card>
      </div>

      <mat-card class="chart-container">
        <mat-card-header class="chart-header">
          <mat-card-title>Financial Allocation by Category</mat-card-title>
          <mat-chip highlighted color="primary" class="chart-engine-tag">
            Chart.js 4 + Moment {{ momentVersion }}
          </mat-chip>
        </mat-card-header>
        <mat-card-content class="canvas-wrapper">
          <canvas #chartCanvas></canvas>
        </mat-card-content>
      </mat-card>
    </div>
  `,
  styles: [`
    .executive-hub { display: flex; flex-direction: column; gap: 1.5rem; }
    .kpi-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 1rem; }
    .kpi-card { background: #ffffff; border: 1px solid #e2e8f0; border-radius: 8px; padding: 1.25rem; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
    .kpi-label { font-size: 0.8rem; font-weight: 600; text-transform: uppercase; color: #64748b; letter-spacing: 0.05em; display: block; }
    .kpi-value { font-size: 1.75rem; font-weight: 700; color: #0f172a; margin: 0.35rem 0; display: block; }
    .kpi-sub { font-size: 0.75rem; color: #94a3b8; }
    .kpi-sub.highlight { color: #10b981; font-weight: 600; }
    .chart-container { background: #ffffff; border: 1px solid #e2e8f0; border-radius: 8px; padding: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
    .chart-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
    .chart-header h3 { margin: 0; font-size: 1.1rem; color: #1e293b; }
    .chart-engine-tag { font-size: 0.75rem; color: #6366f1; background: #e0e7ff; padding: 0.2rem 0.6rem; border-radius: 9999px; font-weight: 600; }
    .canvas-wrapper { position: relative; height: 320px; width: 100%; }
  `]
})
export class ExecutiveChartComponent implements AfterViewInit, OnDestroy {
  @ViewChild('chartCanvas') canvasRef!: ElementRef<HTMLCanvasElement>;
  readonly dataset = inject(ReportDatasetService);
  private chartInstance: Chart | null = null;
  readonly lastUpdated = moment().format('MMM D, YYYY h:mm A');
  readonly momentVersion = moment.version;

  constructor() {
    effect(() => {
      const rows = this.dataset.rows();
      if (this.chartInstance) {
        this.updateChartData(rows);
      }
    });
  }

  ngAfterViewInit() {
    this.initChart();
  }

  ngOnDestroy() {
    if (this.chartInstance) {
      this.chartInstance.destroy();
    }
  }

  private initChart() {
    const ctx = this.canvasRef.nativeElement.getContext('2d');
    if (!ctx) return;

    const rows = this.dataset.rows();
    const categories = rows.map(r => r.category);
    const amounts = rows.map(r => r.amount);

    this.chartInstance = new Chart(ctx, {
      type: 'bar',
      data: {
        labels: categories,
        datasets: [{
          label: 'Item Amount ($)',
          data: amounts,
          backgroundColor: '#4f46e5',
          borderRadius: 4
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false }
        },
        scales: {
          y: {
            beginAtZero: true,
            ticks: {
              callback: (val) => '$' + Number(val).toLocaleString()
            }
          }
        }
      }
    });
  }

  private updateChartData(rows: any[]) {
    if (!this.chartInstance) return;
    this.chartInstance.data.labels = rows.map(r => r.category);
    this.chartInstance.data.datasets[0].data = rows.map(r => r.amount);
    this.chartInstance.update();
  }
}
