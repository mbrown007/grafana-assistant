export interface ParsedDashboardUrl {
  uid: string;
  timeFrom?: string;
  timeTo?: string;
  variables: Record<string, string>;
}

export interface ParsedExploreUrl {
  datasource?: string;
  timeFrom?: string;
  timeTo?: string;
  queries: string[];
}

export function parseDashboardUrl(rawUrl: string): ParsedDashboardUrl | null {
  let url: URL;
  try {
    url = new URL(rawUrl, window.location.origin);
  } catch (error) {
    return null;
  }

  const segments = url.pathname.split('/').filter(Boolean);
  let uid = '';
  for (let i = 0; i < segments.length; i += 1) {
    if (segments[i] === 'd' && segments[i + 1]) {
      uid = segments[i + 1];
      break;
    }
  }
  if (!uid) {
    return null;
  }

  const variables: Record<string, string> = {};
  url.searchParams.forEach((value, key) => {
    if (key.startsWith('var-')) {
      variables[key.replace('var-', '')] = value;
    }
  });

  return {
    uid,
    timeFrom: url.searchParams.get('from') || undefined,
    timeTo: url.searchParams.get('to') || undefined,
    variables,
  };
}

export function parseExploreUrl(rawUrl: string): ParsedExploreUrl | null {
  let url: URL;
  try {
    url = new URL(rawUrl, window.location.origin);
  } catch (error) {
    return null;
  }

  const segments = url.pathname.split('/').filter(Boolean);
  if (!segments.includes('explore')) {
    return null;
  }

  const panesParam = url.searchParams.get('panes');
  if (!panesParam) {
    return null;
  }

  let panes: Record<string, any>;
  try {
    panes = JSON.parse(panesParam);
  } catch (error) {
    return null;
  }

  const queries: string[] = [];
  let datasource: string | undefined;
  let timeFrom: string | undefined;
  let timeTo: string | undefined;

  for (const pane of Object.values(panes)) {
    if (!datasource && typeof pane?.datasource === 'string') {
      datasource = pane.datasource;
    } else if (!datasource && pane?.datasource?.uid) {
      datasource = pane.datasource.uid;
    }

    if (!timeFrom && pane?.range?.from) {
      timeFrom = pane.range.from;
    }
    if (!timeTo && pane?.range?.to) {
      timeTo = pane.range.to;
    }

    if (Array.isArray(pane?.queries)) {
      for (const query of pane.queries) {
        if (typeof query?.expr === 'string' && query.expr.trim()) {
          queries.push(query.expr.trim());
        }
      }
    }
  }

  return {
    datasource,
    timeFrom,
    timeTo,
    queries,
  };
}
