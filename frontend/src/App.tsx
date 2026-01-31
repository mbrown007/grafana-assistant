import React, { useEffect, useRef, useState } from 'react';
import { ChatPanel } from './components/ChatPanel';
import type { DashboardContext } from './types';
import { parseDashboardUrl } from './utils/dashboard';
import chatLauncher from './assets/chat_with_agent.png';

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
  const lastUrlRef = useRef<string>('');

  useEffect(() => {
    let isMounted = true;
    const updateContextFromUrl = async (rawUrl: string) => {
      const parsed = parseDashboardUrl(rawUrl);
      if (!parsed) {
        return;
      }

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
      <main className="app-main">
        <section className="grafana-pane">
          <iframe ref={iframeRef} title="Grafana" src={GRAFANA_PATH} />
          {!sidebarOpen && (
            <button
              type="button"
              className="chat-launcher"
              onClick={() => setSidebarOpen(true)}
              aria-label="Show chat"
              title="Show chat"
            >
              <img src={chatLauncher} alt="Chat with agent" />
            </button>
          )}
        </section>
        <aside className={`chat-pane ${sidebarOpen ? 'open' : 'closed'}`}>
          <ChatPanel dashboardContext={dashboardContext} onHide={() => setSidebarOpen(false)} />
        </aside>
      </main>
    </div>
  );
}
