import type { ReactElement } from 'react';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ChatPanel } from '../ChatPanel';
import { ThemeProvider } from '../ThemeProvider';

// Mock the API module.
vi.mock('../../services/api', () => ({
  chatApi: {
    stream: vi.fn(async function* () {
      yield { type: 'start', session_id: 'test-session' };
      yield { type: 'token', message: 'Hello from AI' };
      yield { type: 'complete', message: 'Hello from AI' };
      yield { type: 'done' };
    }),
  },
  feedbackApi: {
    submit: vi.fn().mockResolvedValue(undefined),
  },
  historyApi: {
    list: vi.fn().mockResolvedValue([]),
    get: vi.fn(),
    remove: vi.fn(),
  },
  userApi: {
    get: vi.fn().mockResolvedValue({ id: 1, login: 'admin', name: 'Admin', org_id: 1 }),
  },
}));

describe('ChatPanel', () => {
  const renderWithTheme = (ui: ReactElement) => render(<ThemeProvider>{ui}</ThemeProvider>);

  it('shows empty state with suggestions when authenticated', async () => {
    renderWithTheme(<ChatPanel />);
    await waitFor(() => {
      expect(screen.getByText("Hi, I'm Flavio")).toBeInTheDocument();
    });
    expect(screen.getByText('Summarize what this dashboard is showing')).toBeInTheDocument();
    expect(screen.getByText('Call out anomalies over the selected time range')).toBeInTheDocument();
    expect(screen.getByText('Which panels look risky right now?')).toBeInTheDocument();
  });

  it('shows waiting state when user is not logged in', async () => {
    const { userApi } = await import('../../services/api');
    vi.mocked(userApi.get).mockRejectedValueOnce(new Error('unauthorized'));

    renderWithTheme(<ChatPanel />);
    await waitFor(() => {
      expect(screen.getByText('Waiting for Grafana login')).toBeInTheDocument();
    });
  });

  it('enables input and submit button when authenticated', async () => {
    renderWithTheme(<ChatPanel />);

    // Wait for auth to complete.
    await waitFor(() => {
      expect(screen.getByText("Hi, I'm Flavio")).toBeInTheDocument();
    });

    const input = screen.getByPlaceholderText('Ask about this dashboard');
    expect(input).not.toBeDisabled();
  });

  it('clicking a suggestion fills the input', async () => {
    renderWithTheme(<ChatPanel />);

    await waitFor(() => {
      expect(screen.getByText("Hi, I'm Flavio")).toBeInTheDocument();
    });

    const suggestion = screen.getByText('Summarize what this dashboard is showing');
    fireEvent.click(suggestion);

    const input = screen.getByPlaceholderText('Ask about this dashboard');
    expect(input).toHaveValue('Summarize what this dashboard is showing');
  });

  it('displays context label with dashboard info', async () => {
    renderWithTheme(
      <ChatPanel
        dashboardContext={{
          uid: 'abc',
          name: 'My Dashboard',
          time_range: { from: 'now-1h', to: 'now' },
        }}
      />
    );

    await waitFor(() => {
      expect(screen.getByText(/My Dashboard/)).toBeInTheDocument();
      expect(screen.getByText(/now-1h/)).toBeInTheDocument();
    });
  });

  it('shows no dashboard context label when none provided', async () => {
    renderWithTheme(<ChatPanel />);
    await waitFor(() => {
      expect(screen.getByText(/No dashboard context yet/)).toBeInTheDocument();
    });
  });

  it('navigates when scratchpad tool returns a URL', async () => {
    const { chatApi } = await import('../../services/api');
    vi.mocked(chatApi.stream).mockImplementationOnce(async function* () {
      yield {
        type: 'tool',
        tool: 'scratchpad__upsert_panel',
        tool_id: 'tool-1',
        result: { url: '/grafana/d/dash-1' },
      };
      yield { type: 'complete', message: 'Done' };
      yield { type: 'done' };
    });

    const onNavigate = vi.fn();
    const { container } = renderWithTheme(<ChatPanel onNavigate={onNavigate} />);

    await waitFor(() => {
      expect(screen.getByText("Hi, I'm Flavio")).toBeInTheDocument();
    });

    const input = screen.getByPlaceholderText('Ask about this dashboard');
    await userEvent.type(input, 'show me cpu');
    const submit = container.querySelector('form button[type="submit"]');
    expect(submit).not.toBeNull();
    await act(async () => {
      fireEvent.click(submit as HTMLButtonElement);
    });

    await waitFor(() => {
      expect(vi.mocked(chatApi.stream)).toHaveBeenCalled();
      expect(onNavigate).toHaveBeenCalledWith('/grafana/d/dash-1');
    });
    expect(onNavigate).toHaveBeenCalledTimes(1);
  });

  it('shows evidence chips and opens modal', async () => {
    const { chatApi } = await import('../../services/api');
    vi.mocked(chatApi.stream).mockImplementationOnce(async function* () {
      yield { type: 'start', session_id: 'test-session' };
      yield {
        type: 'tool',
        tool: 'grafana__query_prometheus',
        tool_id: 'tool-1',
        arguments: { query: 'up' },
        result: { data: [] },
      };
      yield {
        type: 'evidence',
        evidence: {
          kb_search: {
            query: 'what is this',
            results: [{ path: 'docs/overview.md', excerpt: 'overview excerpt', score: 0.9 }],
          },
          vector_search: {
            query: 'what is this',
            results: [{ path: 'docs/vector.md', excerpt: 'vector excerpt', score: 0.8 }],
          },
        },
      };
      yield { type: 'complete', message: 'Done' };
      yield { type: 'done' };
    });

    const { container } = renderWithTheme(<ChatPanel />);

    await waitFor(() => {
      expect(screen.getByText("Hi, I'm Flavio")).toBeInTheDocument();
    });

    const input = screen.getByPlaceholderText('Ask about this dashboard');
    await userEvent.type(input, 'show evidence');
    const submit = container.querySelector('form button[type="submit"]');
    expect(submit).not.toBeNull();
    await act(async () => {
      fireEvent.click(submit as HTMLButtonElement);
    });

    await waitFor(() => {
      expect(screen.getByText('Tool calls (1)')).toBeInTheDocument();
      expect(screen.getByText('KB search')).toBeInTheDocument();
      expect(screen.getByText('Vector search')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole('button', { name: 'KB search' }));
    await waitFor(() => {
      expect(screen.getByText('overview excerpt')).toBeInTheDocument();
    });
  });

  it('submits feedback for an assistant reply', async () => {
    const { chatApi, feedbackApi } = await import('../../services/api');
    vi.mocked(chatApi.stream).mockImplementationOnce(async function* () {
      yield { type: 'start', session_id: 'test-session' };
      yield { type: 'complete', message: 'Done' };
      yield { type: 'done' };
    });

    const { container } = renderWithTheme(<ChatPanel />);

    await waitFor(() => {
      expect(screen.getByText("Hi, I'm Flavio")).toBeInTheDocument();
    });

    const input = screen.getByPlaceholderText('Ask about this dashboard');
    await userEvent.type(input, 'show feedback');
    const submit = container.querySelector('form button[type="submit"]');
    expect(submit).not.toBeNull();
    await act(async () => {
      fireEvent.click(submit as HTMLButtonElement);
    });

    await waitFor(() => {
      expect(screen.getByText('Feedback')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('Feedback'));
    await userEvent.click(screen.getByLabelText('Rate 5 stars'));
    await userEvent.type(screen.getByPlaceholderText('What was helpful or missing?'), 'Nice answer');
    await userEvent.click(screen.getByText('Send feedback'));

    await waitFor(() => {
      expect(vi.mocked(feedbackApi.submit)).toHaveBeenCalledWith(
        expect.objectContaining({
          session_id: 'test-session',
          rating: 5,
          comment: 'Nice answer',
        })
      );
    });
  });
});
