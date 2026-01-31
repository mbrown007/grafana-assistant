export interface ParsedDashboardUrl {
  uid: string;
  timeFrom?: string;
  timeTo?: string;
  variables: Record<string, string>;
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
