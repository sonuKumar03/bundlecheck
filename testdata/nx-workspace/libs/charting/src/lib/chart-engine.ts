import { Chart } from 'chart.js';
import * as d3 from 'd3';
import moment from 'moment';

export class ChartEngine {
  render(elementId: string, data: number[]) {
    console.log('Rendering chart with D3 & Chart.js', d3.scaleLinear(), Chart.name);
    console.log('Created on:', moment().format('YYYY-MM-DD'));
  }
}

export function formatTimestamp(date: Date): string {
  return moment(date).format('MMMM Do YYYY, h:mm:ss a');
}
