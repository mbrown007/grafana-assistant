import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ChatPanel } from '../ChatPanel';

// Mock the API module.
vi.mock('../../services/api', () => ({
  chatApi: {
    async *stream() {
      yield { type: 'start', session_id: 'test-session' };
      yield { type: 'token', message: 'Hello from AI' };
      yield { type: 'complete', message: 'Hello from AI' };
      yield { type: 'done' };
    },
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
  it('shows empty state with suggestions when authenticated', async () => {
    render(<ChatPanel />);
    await waitFor(() => {
      expect(screen.getByText('Ask about this dashboard')).toBeInTheDocument();
    });
    expect(screen.getByText('Summarize what this dashboard is showing')).toBeInTheDocument();
    expect(screen.getByText('Call out anomalies over the selected time range')).toBeInTheDocument();
    expect(screen.getByText('Which panels look risky right now?')).toBeInTheDocument();
  });

  it('shows waiting state when user is not logged in', async () => {
    const { userApi } = await import('../../services/api');
    vi.mocked(userApi.get).mockRejectedValueOnce(new Error('unauthorized'));

    render(<ChatPanel />);
    await waitFor(() => {
      expect(screen.getByText('Waiting for Grafana login')).toBeInTheDocument();
    });
  });

  it('enables input and submit button when authenticated', async () => {
    render(<ChatPanel />);

    // Wait for auth to complete.
    await waitFor(() => {
      expect(screen.getByText('Ask about this dashboard')).toBeInTheDocument();
    });

    const input = screen.getByPlaceholderText('Ask about this dashboard...');
    expect(input).not.toBeDisabled();
  });

  it('clicking a suggestion fills the input', async () => {
    render(<ChatPanel />);

    await waitFor(() => {
      expect(screen.getByText('Ask about this dashboard')).toBeInTheDocument();
    });

    const suggestion = screen.getByText('Summarize what this dashboard is showing');
    fireEvent.click(suggestion);

    const input = screen.getByPlaceholderText('Ask about this dashboard...');
    expect(input).toHaveValue('Summarize what this dashboard is showing');
  });

  it('displays context label with dashboard info', async () => {
    render(
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
    render(<ChatPanel />);
    await waitFor(() => {
      expect(screen.getByText(/No dashboard context yet/)).toBeInTheDocument();
    });
  });
});
