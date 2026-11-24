// Grouping is a presentation of complete report rows, never a replacement for
// them. Unknown/pre-migration identities remain separate rather than guessed.
export function groupCrashes(reports) {
	const groups = new Map();
	for (const report of reports) {
		// Match the API's existing project normalization for retained history.
		const rawProject = (report.Project || '').trim();
		const project = rawProject.toLowerCase() === 'cursemark' ? 'cursemark' : rawProject;
		const key = JSON.stringify([project, report.GroupKey || `report:${report.ID}`]);
		if (!groups.has(key)) groups.set(key, { ...report, Reports: [], Count: 0, LegacyCount: 0, LegacyReportCount: 0, LastIngestedAt: null });
		const group = groups.get(key);
		group.Reports.push(report);
		const count = report.Count ?? 0;
		group.Count += count;
		if (report.OccurrenceVersion !== 1) { group.LegacyCount += count; group.LegacyReportCount += 1; }
		if (report.IngestedAt && (!group.LastIngestedAt || report.IngestedAt > group.LastIngestedAt)) group.LastIngestedAt = report.IngestedAt;
	}
	return [...groups.values()];
}

// A presentation-only family: retained identities remain unchanged on the server.
export function groupCrashList(reports) {
 const normal = [], access = new Map();
 for (const report of reports) {
  if (!(report.Message || '').toLowerCase().includes('access violation')) { normal.push(report); continue; }
  const project = (report.Project || '').trim().toLowerCase();
  if (!access.has(project)) access.set(project, []);
  access.get(project).push(report);
 }
 const groups = groupCrashes(normal);
 for (const [project, members] of access) {
  const subgroups = groupCrashes(members);
  groups.push({...members[0], Message:'Access violations', IsAccessViolationFamily:true,
   ListKey:`access-violations:${project}`, Reports:members,
   Count:subgroups.reduce((n,g)=>n+g.Count,0), LegacyCount:subgroups.reduce((n,g)=>n+g.LegacyCount,0),
   LegacyReportCount:subgroups.reduce((n,g)=>n+g.LegacyReportCount,0),
   LastIngestedAt:subgroups.map(g=>g.LastIngestedAt).filter(Boolean).sort().pop() || null,
   ResolutionIDs:subgroups.map(g=>g.ID), GroupKey:undefined, GroupIdentity:undefined
  });
 }
 return groups;
}
export function crashResolutionIDs(group) { return group.ResolutionIDs || [group.ID]; }
