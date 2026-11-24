const crashDetailsStart = /^\s*(?:GPU Model|GPU Memory|Tracked GPU Memory|Window|Render Surface|Scene|Level|Cursemark|ScaleGrid Guard|Physics Contact Guard)\s*:/im;

export function parseCrashLog(crash, {includeGuards=false} = {}) {
		const rawStack = (crash.Stack || '').replace(/\r\n/g, '\n');
		const match = crashDetailsStart.exec(rawStack);
		let stackText = rawStack;
		let detailsText = '';

		if (match) {
			stackText = rawStack.slice(0, match.index).trim();
			detailsText = rawStack.slice(match.index).trim();
		} else {
			const separator = rawStack.indexOf('\n\n');
			if (separator >= 0) {
				stackText = rawStack.slice(0, separator).trim();
				detailsText = rawStack.slice(separator + 2).trim();
			}
		}

		let frames;
		if (stackText.includes('FilePos(')) {
			frames = stackText
				.split(/,\s*(?=FilePos\()/)
				.map(formatFilePosFrame)
				.filter(Boolean);
		} else {
			frames = stackText
				.split('\n')
				.map(line => line.trim())
				.filter(Boolean);
		}

		const details = detailsText
			.split('\n')
			.map(line => line.trim())
			.filter(Boolean)
			.map(line => {
				const separator = line.indexOf(':');
				return separator < 0
					? { label: 'Additional info', value: line }
					: { label: line.slice(0, separator).trim(), value: line.slice(separator + 1).trim() };
			});

		return { frames, details: includeGuards ? details : details.filter(detail => !/guard/i.test(detail.label)) };
	}

	function formatFilePosFrame(frame) {
		const value = frame.replace(/^\[|\]$/g, '').trim();
		const match = value.match(/^FilePos\(Method\((.*?),(.*?)\),([^,]+),(\d+),.*\)$/);
		if (match) {
			return `${match[1]}.${match[2]} â€” ${match[3]}:${match[4]}`;
		}
		return value.replace(/^FilePos\(/, '').replace(/\)$/, '').replaceAll(',', ' · ');
	}


export function visibleCrashStack(crash) {
 const raw=(crash.Stack||'').replace(/\r\n/g,'\n');
 const match=crashDetailsStart.exec(raw);
 const boundary=match?match.index:raw.indexOf('\n\n');
 if(boundary<0)return raw;
 return raw.slice(0,boundary)+raw.slice(boundary).split('\n').filter(line=>!/^\s*[^:]*guard[^:]*:/i.test(line)).join('\n');
}
