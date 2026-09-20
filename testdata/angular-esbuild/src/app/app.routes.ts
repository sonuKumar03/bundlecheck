import { Component } from '@angular/core';
import { Routes } from '@angular/router';

@Component({ template: `<p>Initial home component</p>` })
class Home {}

export const routes: Routes = [
  { path: '', pathMatch: 'full', component: Home },
  { path: 'lazy', loadComponent: () => import('./lazy').then(m => m.Lazy) },
];
