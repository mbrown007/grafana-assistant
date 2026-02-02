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
});
