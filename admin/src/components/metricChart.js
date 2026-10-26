import {
	Chart, BarController, BarElement, LineController, LineElement, PointElement,
	PieController, ArcElement, CategoryScale, LinearScale, Legend, Tooltip
} from 'chart.js';

Chart.register(BarController, BarElement, LineController, LineElement, PointElement,
	PieController, ArcElement, CategoryScale, LinearScale, Legend, Tooltip);

const colors = ['#3d7058', '#bc6344', '#6289af', '#b39439', '#8f6bb0', '#49a69f', '#cc7794', '#859543', '#8e7560', '#63748c'];
export const label = value => String(value).replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());

export function chartConfig(card, result) {
	const s = card.settings;
	const index = name => result.columns.indexOf(name);
	const dimension = s['graph.dimensions'] || [];
	const xName = card.display === 'pie' ? s['pie.dimension'][0] : dimension[0];
	const metrics = card.display === 'pie' ? [s['pie.metric']] : s['graph.metrics'];
	const xIndex = index(xName);
	const splitIndex = dimension.length > 1 ? index(dimension[1]) : -1;
	const numeric = s['graph.x_axis.scale'] === 'linear';
	const labels = [...new Set(result.rows.map(row => row[xIndex]))];
	const splits = splitIndex >= 0 ? [...new Set(result.rows.map(row => row[splitIndex]))] : [null];
	const datasets = splits.flatMap(split => metrics.map(metric => {
		const rows = splitIndex >= 0 ? result.rows.filter(row => row[splitIndex] === split) : result.rows;
		const values = new Map(rows.map(row => [row[xIndex], row[index(metric)]]));
		return {
			label: split === null ? label(metric) : String(split),
			data: numeric ? rows.map(row => ({ x: Number(row[xIndex]), y: row[index(metric)] === null ? null : Number(row[index(metric)]) }))
				: labels.map(x => values.get(x) == null ? null : Number(values.get(x))),
			borderWidth: card.display === 'line' ? 2 : 1,
			pointRadius: rows.length > 80 ? 0 : 2,
			spanGaps: false
		};
	}));
	datasets.forEach((dataset, i) => {
		dataset.borderColor = card.display === 'pie' ? '#fff' : colors[i % colors.length];
		dataset.backgroundColor = card.display === 'pie' ? labels.map((_, j) => colors[j % colors.length]) : colors[i % colors.length];
	});
	return {
		type: card.display,
		data: { labels, datasets },
		options: {
			responsive: true, maintainAspectRatio: false, animation: false,
			plugins: {
				legend: { display: card.display === 'pie' || datasets.length > 1, position: 'bottom', labels: { boxWidth: 12, font: { size: 11 } } },
				tooltip: { callbacks: card.display === 'pie' ? { label: context => `${context.label}: ${context.formattedValue} players` } : {} }
			},
			...(card.display === 'pie' ? {} : { scales: {
				x: { type: numeric ? 'linear' : 'category', title: { display: true, text: label(xName) }, ticks: { maxRotation: 60, autoSkip: true } },
				y: { beginAtZero: true, title: { display: metrics.length === 1, text: label(metrics[0]) } }
			} })
		}
	};
}

export function metricChart(node, { card, result }) {
	let chart = new Chart(node, chartConfig(card, result));
	return { destroy() { chart.destroy(); } };
}
