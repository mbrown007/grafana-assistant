export function getBasePath(): string {
  if (typeof window === 'undefined') {
    return '';
  }
  const raw = window.__ASSISTANT_BASE_PATH__ ?? '';
  const trimmed = raw.trim();
  if (!trimmed || trimmed === '/') {
    return '';
  }
  const normalized = trimmed.startsWith('/') ? trimmed : `/${trimmed}`;
  return normalized.replace(/\/+$/, '');
}

export function withBasePath(path: string): string {
  const base = getBasePath();
  if (!base) {
    return path;
  }
  if (!path) {
    return base;
  }
  if (path === base || path.startsWith(base + '/')) {
    return path;
  }
  const normalized = path.startsWith('/') ? path : `/${path}`;
  return `${base}${normalized}`;
}
