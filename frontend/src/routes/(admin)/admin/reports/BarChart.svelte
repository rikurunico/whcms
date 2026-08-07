<script lang="ts">
	/**
	 * Single-series SVG bar chart (no chart libs, per FRONTEND.md §1).
	 * Mark spec: bars ≤ 24px thick, 4px rounded data-end (square baseline),
	 * ≥ 2px surface gap between bars, hairline gridlines, per-mark hover tooltip.
	 * Single series -> no legend (the section title names it). HostPanel palette:
	 * primary #337ab7 bar fill, navy #1a4d80 on hover.
	 */
	interface Point {
		label: string;
		value: number;
	}

	interface Props {
		data: Point[];
		/** Full-precision formatter for tooltips/direct label. */
		formatValue?: (v: number) => string;
		/** Compact formatter for y-axis ticks. */
		formatTick?: (v: number) => string;
		height?: number;
		testid?: string;
	}

	const defaultFormat = (v: number) => new Intl.NumberFormat('id-ID').format(v);
	const defaultTick = (v: number) =>
		new Intl.NumberFormat('id-ID', { notation: 'compact', maximumFractionDigits: 1 }).format(v);

	let {
		data,
		formatValue = defaultFormat,
		formatTick = defaultTick,
		height = 260,
		testid
	}: Props = $props();

	let width = $state(720);
	let hovered = $state<number | null>(null);

	const margin = { top: 20, right: 12, bottom: 26, left: 56 };
	const innerW = $derived(Math.max(10, width - margin.left - margin.right));
	const innerH = $derived(height - margin.top - margin.bottom);

	/** Round the axis max up to a clean 1/2/2.5/5×10^n number. */
	function niceMax(maxValue: number): number {
		if (maxValue <= 0) return 1;
		const pow = Math.pow(10, Math.floor(Math.log10(maxValue)));
		for (const m of [1, 2, 2.5, 5, 10]) {
			if (m * pow >= maxValue) return m * pow;
		}
		return 10 * pow;
	}

	const maxValue = $derived(data.reduce((m, d) => Math.max(m, d.value), 0));
	const yMax = $derived(niceMax(maxValue));
	const maxIndex = $derived(data.reduce((mi, d, i) => (d.value > data[mi].value ? i : mi), 0));

	const band = $derived(data.length > 0 ? innerW / data.length : innerW);
	const barW = $derived(Math.max(2, Math.min(24, band - 2)));

	function x(i: number): number {
		return margin.left + i * band + (band - barW) / 2;
	}

	function barH(v: number): number {
		return yMax === 0 ? 0 : (v / yMax) * innerH;
	}

	/** Bar path: 4px rounded top corners, square baseline. */
	function barPath(i: number, v: number): string {
		const h = barH(v);
		const bx = x(i);
		const by = margin.top + innerH - h;
		const r = Math.min(4, barW / 2, h);
		if (h <= 0) return '';
		if (r <= 0.5) return `M${bx},${by + h} V${by} H${bx + barW} V${by + h} Z`;
		return [
			`M${bx},${by + h}`,
			`V${by + r}`,
			`a${r},${r} 0 0 1 ${r},-${r}`,
			`H${bx + barW - r}`,
			`a${r},${r} 0 0 1 ${r},${r}`,
			`V${by + h}`,
			'Z'
		].join(' ');
	}

	const gridLevels = [0.25, 0.5, 0.75, 1];
	/** Show at most ~8 x labels to avoid collisions. */
	const labelStep = $derived(Math.max(1, Math.ceil(data.length / 8)));

	const tooltip = $derived.by(() => {
		if (hovered === null || !data[hovered]) return null;
		const d = data[hovered];
		return {
			left: x(hovered) + barW / 2,
			top: margin.top + innerH - barH(d.value),
			label: d.label,
			value: formatValue(d.value)
		};
	});
</script>

<div class="hp-chart" bind:clientWidth={width} data-testid={testid}>
	{#if data.length === 0}
		<p class="hp-chart-empty">No data available</p>
	{:else}
		<svg
			{width}
			{height}
			viewBox={`0 0 ${width} ${height}`}
			role="img"
			aria-label="Report chart"
			onpointerleave={() => (hovered = null)}
		>
			<!-- hairline gridlines + y ticks -->
			{#each gridLevels as level (level)}
				{@const gy = margin.top + innerH - level * innerH}
				<line
					x1={margin.left}
					x2={margin.left + innerW}
					y1={gy}
					y2={gy}
					stroke="#eee"
					stroke-width="1"
				/>
				<text
					x={margin.left - 8}
					y={gy + 3}
					text-anchor="end"
					font-size="10"
					fill="#888"
					style="font-variant-numeric: tabular-nums"
				>
					{formatTick(level * yMax)}
				</text>
			{/each}

			<!-- bars -->
			{#each data as d, i (i)}
				<path
					d={barPath(i, d.value)}
					fill={hovered === i ? 'var(--hp-navy, #1a4d80)' : 'var(--hp-primary, #337ab7)'}
				/>
			{/each}

			<!-- baseline -->
			<line
				x1={margin.left}
				x2={margin.left + innerW}
				y1={margin.top + innerH}
				y2={margin.top + innerH}
				stroke="#ccc"
				stroke-width="1"
			/>

			<!-- selective direct label: the max bar only -->
			{#if maxValue > 0 && hovered === null}
				<text
					x={x(maxIndex) + barW / 2}
					y={margin.top + innerH - barH(data[maxIndex].value) - 5}
					text-anchor="middle"
					font-size="10"
					font-weight="600"
					fill="#555"
					style="font-variant-numeric: tabular-nums"
				>
					{formatValue(data[maxIndex].value)}
				</text>
			{/if}

			<!-- x labels (thinned) -->
			{#each data as d, i (i)}
				{#if i % labelStep === 0}
					<text x={x(i) + barW / 2} y={height - 8} text-anchor="middle" font-size="10" fill="#888">
						{d.label}
					</text>
				{/if}
			{/each}

			<!-- full-band hover/focus hit targets (bigger than the mark) -->
			{#each data as d, i (i)}
				<!-- svelte-ignore a11y_no_noninteractive_tabindex -- deliberate: keyboard focus reveals the same tooltip as hover -->
				<rect
					x={margin.left + i * band}
					y={margin.top}
					width={band}
					height={innerH}
					fill="transparent"
					tabindex="0"
					role="graphics-symbol"
					aria-label={`${d.label}: ${formatValue(d.value)}`}
					onpointerenter={() => (hovered = i)}
					onfocus={() => (hovered = i)}
					onblur={() => (hovered = null)}
				/>
			{/each}
		</svg>

		{#if tooltip}
			<div
				class="hp-chart-tip"
				style={`left:${tooltip.left}px; top:${Math.max(0, tooltip.top - 6)}px`}
			>
				<p class="hp-chart-tip-val">{tooltip.value}</p>
				<p class="hp-chart-tip-lbl">{tooltip.label}</p>
			</div>
		{/if}
	{/if}
</div>

<style>
	.hp-chart {
		position: relative;
	}
	.hp-chart-empty {
		padding: 48px 0;
		text-align: center;
		font-size: 13px;
		color: #999;
	}
	.hp-chart-tip {
		position: absolute;
		z-index: 10;
		pointer-events: none;
		transform: translate(-50%, -100%);
		border: 1px solid #ddd;
		border-radius: 4px;
		background: #fff;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
		padding: 6px 10px;
		font-size: 12px;
	}
	.hp-chart-tip-val {
		margin: 0;
		font-weight: 700;
		color: #333;
		font-variant-numeric: tabular-nums;
	}
	.hp-chart-tip-lbl {
		margin: 2px 0 0;
		color: #888;
	}
</style>
