import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { AuthService } from './auth.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    MatButtonModule,
    MatIconModule
  ],
  template: `
    <div class="login-viewport">
      <mat-card class="login-card">
        <div class="login-header">
          <div class="brand-badge">OR</div>
          <h2 mat-card-title>OmniReport Studio</h2>
          <p class="subtitle">Enterprise Document & Intelligence Platform</p>
        </div>

        <mat-card-content>
          <form (ngSubmit)="onLogin()" class="login-form">
            <mat-form-field appearance="outline" class="full-width">
              <mat-label>Enterprise ID or Email</mat-label>
              <input
                matInput
                type="text"
                name="username"
                [(ngModel)]="username"
                placeholder="e.g. auditor@omnireport.internal"
                required
              />
              <mat-icon matSuffix>account_circle</mat-icon>
            </mat-form-field>

            <mat-form-field appearance="outline" class="full-width">
              <mat-label>Password</mat-label>
              <input
                matInput
                type="password"
                name="password"
                [(ngModel)]="password"
                placeholder="••••••••••••"
                required
              />
              <mat-icon matSuffix>lock</mat-icon>
            </mat-form-field>

            <button
              mat-flat-button
              color="primary"
              type="submit"
              class="btn-submit"
              [disabled]="!username"
            >
              Sign In to Studio
            </button>
          </form>

          <div class="divider">
            <span>Quick Demo Access</span>
          </div>

          <button
            mat-stroked-button
            type="button"
            class="btn-quick full-width"
            (click)="onQuickLogin()"
          >
            <mat-icon>bolt</mat-icon> 1-Click Sign In as Executive Auditor
          </button>

          <div class="security-footer">
            <span>🔒 End-to-End Encrypted Session • Port 3000 • Angular Material</span>
          </div>
        </mat-card-content>
      </mat-card>
    </div>
  `,
  styles: [`
    .login-viewport {
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      background: linear-gradient(135deg, #0f172a 0%, #1e1b4b 50%, #0f172a 100%);
      padding: 1.5rem;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
    }

    .login-card {
      width: 100%;
      max-width: 420px;
      background: #ffffff;
      border-radius: 12px;
      padding: 2.5rem;
      box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.25), 0 10px 10px -5px rgba(0, 0, 0, 0.15);
      border: 1px solid rgba(255, 255, 255, 0.1);
    }

    .login-header {
      text-align: center;
      margin-bottom: 2rem;
    }

    .brand-badge {
      width: 48px;
      height: 48px;
      margin: 0 auto 0.75rem auto;
      background: linear-gradient(135deg, #4f46e5 0%, #06b6d4 100%);
      color: white;
      font-weight: 800;
      border-radius: 10px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 1.35rem;
      box-shadow: 0 4px 6px -1px rgba(79, 70, 229, 0.3);
    }

    .login-header h2 {
      margin: 0;
      font-size: 1.45rem;
      color: #0f172a;
      font-weight: 700;
    }

    .subtitle {
      margin: 0.35rem 0 0 0;
      font-size: 0.8rem;
      color: #64748b;
    }

    .login-form {
      display: flex;
      flex-direction: column;
      gap: 1.15rem;
    }

    .form-group {
      display: flex;
      flex-direction: column;
      gap: 0.35rem;
    }

    .form-group label {
      font-size: 0.8rem;
      font-weight: 600;
      color: #475569;
    }

    .form-group input {
      padding: 0.65rem 0.85rem;
      border: 1px solid #cbd5e1;
      border-radius: 6px;
      font-size: 0.9rem;
      outline: none;
      transition: border-color 0.15s;
    }

    .form-group input:focus {
      border-color: #4f46e5;
      box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.1);
    }

    .btn-primary {
      padding: 0.75rem;
      background: #4f46e5;
      color: white;
      font-weight: 600;
      border: none;
      border-radius: 6px;
      font-size: 0.95rem;
      cursor: pointer;
      margin-top: 0.5rem;
      transition: background 0.15s;
    }

    .btn-primary:hover:not(:disabled) {
      background: #4338ca;
    }

    .btn-primary:disabled {
      opacity: 0.6;
      cursor: not-allowed;
    }

    .divider {
      display: flex;
      align-items: center;
      text-align: center;
      margin: 1.5rem 0;
      color: #94a3b8;
      font-size: 0.75rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    .divider::before, .divider::after {
      content: '';
      flex: 1;
      border-bottom: 1px solid #e2e8f0;
    }

    .divider span {
      padding: 0 0.75rem;
    }

    .btn-quick {
      width: 100%;
      padding: 0.65rem;
      background: #f8fafc;
      border: 1px solid #cbd5e1;
      color: #1e293b;
      font-weight: 600;
      border-radius: 6px;
      font-size: 0.85rem;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 0.4rem;
      transition: all 0.15s;
    }

    .btn-quick:hover {
      background: #f1f5f9;
      border-color: #94a3b8;
    }

    .security-footer {
      margin-top: 1.75rem;
      text-align: center;
      font-size: 0.72rem;
      color: #94a3b8;
    }
  `]
})
export class LoginComponent {
  username = 'auditor@omnireport.internal';
  password = '••••••••••••';
  private readonly auth = inject(AuthService);

  onLogin() {
    this.auth.login(this.username, this.password);
  }

  onQuickLogin() {
    this.auth.quickLogin('auditor');
  }
}
