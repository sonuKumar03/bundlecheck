import { Component } from '@angular/core';
import { format } from 'date-fns';

@Component({ template: `<h2>Lazy report</h2><p>{{ today }}</p>` })
export class Lazy {
  readonly today = format(new Date(), 'yyyy-MM-dd');
}
