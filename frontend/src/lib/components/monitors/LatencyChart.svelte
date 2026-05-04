<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import uPlot from 'uplot';
	import 'uplot/dist/uPlot.min.css';
	import type { MonitorResult } from '$lib/api/monitors';

	type Props = { results: MonitorResult[] };
	let { results }: Props = $props();

	let containerEl: HTMLDivElement;
	let plot: uPlot | null = null;

	function buildData(rs: MonitorResult[]): uPlot.AlignedData {
		const xs: number[] = [];
		const ys: (number | null)[] = [];
		for (const r of rs) {
			xs.push(new Date(r.checked_at).getTime() / 1000);
			ys.push(r.status === 'up' && r.latency_ms != null ? r.latency_ms : null);
		}
		return [xs, ys];
	}

	function makeOpts(width: number): uPlot.Options {
		return {
			width,
			height: 240,
			scales: { x: { time: true } },
			axes: [
				{
					stroke: '#8b949e',
					grid: { stroke: '#30363d', width: 0.5 },
					ticks: { stroke: '#30363d' }
				},
				{
					stroke: '#8b949e',
					grid: { stroke: '#30363d', width: 0.5 },
					ticks: { stroke: '#30363d' },
					label: 'ms'
				}
			],
			series: [
				{},
				{
					label: 'latency',
					stroke: '#7dd3fc',
					width: 1.5,
					spanGaps: false,
					points: { show: false }
				}
			],
			cursor: { drag: { x: false, y: false } },
			legend: { show: false }
		};
	}

	onMount(() => {
		const width = containerEl.clientWidth || 600;
		plot = new uPlot(makeOpts(width), buildData(results), containerEl);
		const ro = new ResizeObserver((entries) => {
			for (const entry of entries) {
				plot?.setSize({ width: entry.contentRect.width, height: 240 });
			}
		});
		ro.observe(containerEl);
		return () => ro.disconnect();
	});

	$effect(() => {
		if (plot) plot.setData(buildData(results));
	});

	onDestroy(() => plot?.destroy());
</script>

<div bind:this={containerEl} class="w-full" style="height: 240px;"></div>
