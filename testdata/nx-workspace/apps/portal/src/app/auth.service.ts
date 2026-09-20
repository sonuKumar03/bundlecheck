import { Injectable, signal, computed } from '@angular/core';
import { Router } from '@angular/router';

export interface UserProfile {
  username: string;
  name: string;
  role: string;
}

const DEFAULT_USER: UserProfile = {
  username: 'auditor@omnireport.internal',
  name: 'Executive Auditor',
  role: 'Senior Financial Analyst'
};

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private readonly _currentUser = signal<UserProfile | null>(DEFAULT_USER);
  readonly currentUser = this._currentUser.asReadonly();
  readonly isAuthenticated = computed(() => !!this._currentUser());

  constructor(private router: Router) {
    const saved = localStorage.getItem('omnireport_user');
    if (saved) {
      try {
        this._currentUser.set(JSON.parse(saved));
      } catch {
        this._currentUser.set(DEFAULT_USER);
      }
    }
  }

  login(username: string, password?: string): boolean {
    if (!username) return false;
    const user: UserProfile = {
      username,
      name: username.includes('@') ? username.split('@')[0] : username,
      role: 'Enterprise Member'
    };
    this._currentUser.set(user);
    localStorage.setItem('omnireport_user', JSON.stringify(user));
    this.router.navigate(['/overview']);
    return true;
  }

  quickLogin(role: 'executive' | 'auditor' = 'auditor'): void {
    const user: UserProfile = role === 'executive' 
      ? { username: 'cfo@omnireport.com', name: 'Chief Financial Officer', role: 'Executive Board' }
      : DEFAULT_USER;
    this._currentUser.set(user);
    localStorage.setItem('omnireport_user', JSON.stringify(user));
    this.router.navigate(['/overview']);
  }

  logout(): void {
    this._currentUser.set(null);
    localStorage.removeItem('omnireport_user');
    this.router.navigate(['/login']);
  }
}
