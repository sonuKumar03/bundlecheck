import { Component, inject } from '@angular/core';
import { RouterModule } from '@angular/router';
import { CommonModule } from '@angular/common';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatChipsModule } from '@angular/material/chips';
// GOTCHA 3: CommonJS lodash import (required for bundlecheck fixtures)
import cloneDeep from 'lodash/cloneDeep';
import { ReportDatasetService } from '@nx-workspace/reports';
import { AuthService } from './auth.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    RouterModule,
    CommonModule,
    MatToolbarModule,
    MatButtonModule,
    MatIconModule,
    MatChipsModule
  ],
  template: `
    <!-- If not authenticated, render pure router-outlet for Login view -->
    <ng-container *ngIf="!auth.isAuthenticated()">
      <router-outlet></router-outlet>
    </ng-container>

    <!-- If authenticated, render full OmniReport Studio Shell -->
    <div class="studio-app" *ngIf="auth.isAuthenticated()">
      <!-- Studio Header -->
      <header class="studio-header">
        <div class="brand">
          <div class="logo-badge">OR</div>
          <div class="brand-text">
            <h1 class="brand-title">OmniReport Studio</h1>
            <span class="brand-sub">Enterprise Document & Intelligence Platform</span>
          </div>
        </div>
        <div class="header-status">
          <span class="port-indicator">
            <span class="status-dot"></span> Serving on port 3000
          </span>
          <span class="dataset-pill">{{ dataset.rows().length }} Active Records</span>
          <div class="user-pill" *ngIf="auth.currentUser() as user">
            <span class="user-avatar">👤</span>
            <span class="user-name">{{ user.name }}</span>
            <button class="btn-logout" (click)="auth.logout()" title="Sign Out">Sign Out</button>
          </div>
        </div>
      </header>

      <!-- Main Studio Shell -->
      <div class="studio-body">
        <!-- Sidebar Navigation -->
        <aside class="studio-sidebar">
          <div class="nav-sections">
            <nav class="nav-group">
              <span class="nav-label">WORKSPACES</span>
              <a routerLink="/overview" routerLinkActive="active" class="nav-item">
                <span class="nav-icon">📊</span>
                <span class="nav-text">Executive Overview</span>
              </a>
              <a routerLink="/sheets" routerLinkActive="active" class="nav-item">
                <span class="nav-icon">📈</span>
                <span class="nav-text">Spreadsheet Studio</span>
              </a>
              <a routerLink="/pdf" routerLinkActive="active" class="nav-item">
                <span class="nav-icon">📑</span>
                <span class="nav-text">PDF Inspector</span>
              </a>
              <a routerLink="/documents" routerLinkActive="active" class="nav-item">
                <span class="nav-icon">📝</span>
                <span class="nav-text">Document Editor</span>
              </a>
            </nav>

            <nav class="nav-group">
              <span class="nav-label">LEGACY AUDIT</span>
              <a routerLink="/reports" routerLinkActive="active" class="nav-item">
                <span class="nav-icon">📁</span>
                <span class="nav-text">Classic Reports</span>
              </a>
            </nav>
          </div>

          <div class="sidebar-footer">
            <div class="package-weights">
              <span class="weight-label">Active Stack:</span>
              <div class="tag-cloud">
                <span class="tech-tag">Chart.js</span>
                <span class="tech-tag">ExcelJS</span>
                <span class="tech-tag">PDF.js</span>
                <span class="tech-tag">Moment</span>
                <span class="tech-tag">Lodash</span>
              </div>
            </div>
          </div>
        </aside>

        <!-- Main Content Canvas -->
        <main class="studio-content">
          <router-outlet></router-outlet>
        </main>
      </div>

      <!-- Studio Status Banner -->
      <footer class="studio-footer">
        <span>OmniReport Enterprise Studio v2.4</span>
        <span>Connected to Nx Workspace (Local Host: 3000)</span>
      </footer>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      height: 100vh;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
      background: #f8fafc;
      color: #0f172a;
      -webkit-font-smoothing: antialiased;
      -moz-osx-font-smoothing: grayscale;
    }

    * {
      box-sizing: border-box;
    }

    .studio-app {
      display: flex;
      flex-direction: column;
      height: 100vh;
      overflow: hidden;
    }

    .studio-header {
      height: 62px;
      background: #ffffff;
      border-bottom: 1px solid #e2e8f0;
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 0 1.5rem;
      flex-shrink: 0;
      box-shadow: 0 1px 2px 0 rgba(0, 0, 0, 0.03);
    }

    .brand {
      display: flex;
      align-items: center;
      gap: 0.85rem;
    }

    .logo-badge {
      width: 38px;
      height: 38px;
      background: linear-gradient(135deg, #4f46e5 0%, #06b6d4 100%);
      color: white;
      font-weight: 800;
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 1.05rem;
      letter-spacing: -0.5px;
      box-shadow: 0 2px 4px rgba(79, 70, 229, 0.25);
    }

    .brand-text {
      display: flex;
      flex-direction: column;
    }

    .brand-title {
      font-size: 1.15rem;
      font-weight: 700;
      margin: 0;
      color: #0f172a;
      line-height: 1.25;
      letter-spacing: -0.02em;
    }

    .brand-sub {
      font-size: 0.72rem;
      color: #64748b;
      font-weight: 500;
    }

    .header-status {
      display: flex;
      align-items: center;
      gap: 1rem;
      font-size: 0.8rem;
    }

    .port-indicator {
      display: inline-flex;
      align-items: center;
      gap: 0.4rem;
      color: #16a34a;
      font-weight: 600;
      font-size: 0.8rem;
      background: #f0fdf4;
      border: 1px solid #bbf7d0;
      padding: 0.25rem 0.65rem;
      border-radius: 9999px;
    }

    .status-dot {
      width: 7px;
      height: 7px;
      background-color: #16a34a;
      border-radius: 50%;
      display: inline-block;
      box-shadow: 0 0 6px #22c55e;
      animation: pulse 2s infinite ease-in-out;
    }

    @keyframes pulse {
      0%, 100% { opacity: 1; transform: scale(1); }
      50% { opacity: 0.5; transform: scale(0.85); }
    }

    .dataset-pill {
      background: #e0e7ff;
      color: #4338ca;
      padding: 0.25rem 0.65rem;
      border-radius: 9999px;
      font-weight: 600;
      font-size: 0.8rem;
      border: 1px solid #c7d2fe;
    }

    .studio-body {
      display: flex;
      flex: 1;
      overflow: hidden;
    }

    .studio-sidebar {
      width: 250px;
      background: #ffffff;
      border-right: 1px solid #e2e8f0;
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      padding: 1.25rem 1rem;
      flex-shrink: 0;
    }

    .nav-sections {
      display: flex;
      flex-direction: column;
      gap: 1.25rem;
    }

    .nav-group {
      display: flex;
      flex-direction: column;
      gap: 0.3rem;
    }

    .nav-label {
      font-size: 0.68rem;
      font-weight: 700;
      color: #94a3b8;
      letter-spacing: 0.06em;
      padding: 0 0.5rem 0.25rem 0.5rem;
    }

    .nav-item {
      display: flex;
      align-items: center;
      gap: 0.65rem;
      padding: 0.55rem 0.75rem;
      border-radius: 6px;
      color: #475569;
      text-decoration: none;
      font-size: 0.88rem;
      font-weight: 500;
      transition: all 0.15s ease-in-out;
    }

    .nav-item:hover {
      background: #f1f5f9;
      color: #0f172a;
    }

    .nav-item.active {
      background: #eef2ff;
      color: #4f46e5;
      font-weight: 600;
    }

    .nav-icon {
      font-size: 1.1rem;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 20px;
    }

    .sidebar-footer {
      border-top: 1px solid #f1f5f9;
      padding-top: 1rem;
    }

    .weight-label {
      font-size: 0.75rem;
      font-weight: 600;
      color: #64748b;
      display: block;
      margin-bottom: 0.5rem;
    }

    .tag-cloud {
      display: flex;
      flex-wrap: wrap;
      gap: 0.35rem;
    }

    .tech-tag {
      font-size: 0.7rem;
      background: #f1f5f9;
      color: #475569;
      padding: 0.15rem 0.45rem;
      border-radius: 4px;
      font-weight: 500;
      border: 1px solid #e2e8f0;
    }

    .studio-content {
      flex: 1;
      overflow-y: auto;
      padding: 1.5rem;
      background: #f8fafc;
    }

    .user-pill {
      display: inline-flex;
      align-items: center;
      gap: 0.5rem;
      background: #f8fafc;
      border: 1px solid #e2e8f0;
      padding: 0.2rem 0.5rem 0.2rem 0.65rem;
      border-radius: 9999px;
      font-size: 0.8rem;
    }

    .user-avatar {
      font-size: 0.9rem;
    }

    .user-name {
      font-weight: 600;
      color: #334155;
    }

    .btn-logout {
      background: #f1f5f9;
      border: 1px solid #cbd5e1;
      border-radius: 9999px;
      padding: 0.15rem 0.5rem;
      font-size: 0.72rem;
      font-weight: 600;
      color: #dc2626;
      cursor: pointer;
      transition: all 0.15s;
    }

    .btn-logout:hover {
      background: #fee2e2;
      border-color: #fca5a5;
    }

    .studio-footer {
      height: 32px;
      background: #ffffff;
      border-top: 1px solid #e2e8f0;
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 0 1.5rem;
      font-size: 0.75rem;
      color: #94a3b8;
      flex-shrink: 0;
    }
  `]
})
export class AppComponent {
  title = 'portal';
  readonly dataset = inject(ReportDatasetService);
  readonly auth = inject(AuthService);

  copyConfig(cfg: any) {
    return cloneDeep(cfg);
  }
}
