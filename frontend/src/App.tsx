import React, { useEffect, useRef, useState } from 'react';
import { ChatPanel } from './components/ChatPanel';
import type { DashboardContext } from './types';
import { parseDashboardUrl } from './utils/dashboard';

const GRAFANA_PATH = '/grafana/';
const CONTEXT_POLL_INTERVAL_MS = 2500;

interface DashboardSummaryResponse {
  uid: string;
  title: string;
  folder?: string;
  tags?: string[];
}

export function App() {
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [dashboardContext, setDashboardContext] = useState<DashboardContext | undefined>();
  const [contextStatus, setContextStatus] = useState('Waiting for dashboard');
  const lastUrlRef = useRef<string>('');

  useEffect(() => {
    let isMounted = true;
    const updateContextFromUrl = async (rawUrl: string) => {
      const parsed = parseDashboardUrl(rawUrl);
      if (!parsed) {
        if (isMounted) {
          setContextStatus('No dashboard detected');
        }
        return;
      }

      setContextStatus('Loading dashboard context');

      let summary: DashboardSummaryResponse | null = null;
      try {
        const response = await fetch(`/api/dashboard-context/${parsed.uid}`);
        if (response.ok) {
          summary = (await response.json()) as DashboardSummaryResponse;
        }
      } catch (error) {
        console.warn('Failed to fetch dashboard context', error);
      }

      if (!isMounted) {
        return;
      }

      setDashboardContext({
        uid: parsed.uid,
        name: summary?.title,
        folder: summary?.folder,
        tags: summary?.tags ?? [],
        time_range: {
          from: parsed.timeFrom ?? '',
          to: parsed.timeTo ?? '',
        },
        variables: parsed.variables,
      });

      setContextStatus(`Dashboard ${summary?.title ?? parsed.uid}`);
    };

    const interval = window.setInterval(() => {
      const iframe = iframeRef.current;
      if (!iframe) {
        return;
      }

      let url = '';
      try {
        url = iframe.contentWindow?.location.href ?? '';
      } catch (error) {
        url = iframe.src;
      }

      if (!url || url === lastUrlRef.current) {
        return;
      }

      lastUrlRef.current = url;
      updateContextFromUrl(url);
    }, CONTEXT_POLL_INTERVAL_MS);

    return () => {
      isMounted = false;
      window.clearInterval(interval);
    };
  }, []);

  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="brand">
          <span className="brand-badge">MA</span>
          <div>
            <div className="brand-title">Monitoring Assistant</div>
            <div className="brand-subtitle">Grafana wrapper + chat</div>
          </div>
        </div>
        <div className="header-controls">
          <div className="context-pill">{contextStatus}</div>
          <button className="toggle-button" onClick={() => setSidebarOpen((prev) => !prev)}>
            {sidebarOpen ? 'Hide Chat' : 'Show Chat'}
          </button>
        </div>
      </header>
      <main className="app-main">
        <section className="grafana-pane">
          <iframe ref={iframeRef} title="Grafana" src={GRAFANA_PATH} />
        </section>
        <aside className={`chat-pane ${sidebarOpen ? 'open' : 'closed'}`}>
          <ChatPanel dashboardContext={dashboardContext} />
        </aside>
      </main>
    </div>
  );
}
