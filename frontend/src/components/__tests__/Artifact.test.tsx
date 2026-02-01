import { render, screen } from '@testing-library/react';
import { Artifact, parseArtifacts } from '../Artifact';
import type { ArtifactData } from '../Artifact';

// Mock recharts to avoid canvas/SVG issues in jsdom.
vi.mock('recharts', () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div data-testid="responsive-container">{children}</div>,
  BarChart: ({ children }: { children: React.ReactNode }) => <div data-testid="bar-chart">{children}</div>,
  LineChart: ({ children }: { children: React.ReactNode }) => <div data-testid="line-chart">{children}</div>,
  PieChart: ({ children }: { children: React.ReactNode }) => <div data-testid="pie-chart">{children}</div>,
  AreaChart: ({ children }: { children: React.ReactNode }) => <div data-testid="area-chart">{children}</div>,
  Bar: () => null,
  Line: () => null,
  Pie: () => null,
  Area: () => null,
  Cell: () => null,
  CartesianGrid: () => null,
  Legend: () => null,
  Tooltip: () => null,
  XAxis: () => null,
  YAxis: () => null,
}));

describe('parseArtifacts', () => {
  it('extracts artifact JSON from markdown', () => {
    const content = 'Some text\n```artifact\n{"type":"table","title":"Test"}\n```\nMore text';
    const { artifacts, remainingContent } = parseArtifacts(content);
    expect(artifacts).toHaveLength(1);
    expect(artifacts[0].type).toBe('table');
    expect(artifacts[0].title).toBe('Test');
    expect(remainingContent).toContain('Some text');
    expect(remainingContent).not.toContain('artifact');
  });

  it('returns empty artifacts for plain content', () => {
    const { artifacts, remainingContent } = parseArtifacts('Just plain text');
    expect(artifacts).toHaveLength(0);
    expect(remainingContent).toBe('Just plain text');
  });

  it('handles invalid JSON gracefully', () => {
    const content = '```artifact\n{invalid json}\n```';
    const { artifacts } = parseArtifacts(content);
    expect(artifacts).toHaveLength(0);
  });
});

describe('Artifact', () => {
  it('renders metric cards', () => {
    const artifact: ArtifactData = {
      type: 'metric-cards',
      title: 'Key Metrics',
      metrics: [
        { label: 'Users', value: 1234, color: 'blue' },
        { label: 'Errors', value: 5, change: -20, color: 'red' },
      ],
    };
    render(<Artifact artifact={artifact} />);
    expect(screen.getByText('Key Metrics')).toBeInTheDocument();
    expect(screen.getByText('Users')).toBeInTheDocument();
    expect(screen.getByText('1234')).toBeInTheDocument();
    expect(screen.getByText('Errors')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
  });

  it('renders a table', () => {
    const artifact: ArtifactData = {
      type: 'table',
      title: 'Server Status',
      columns: [
        { key: 'name', label: 'Name' },
        { key: 'status', label: 'Status' },
      ],
      rows: [
        { name: 'web-01', status: 'healthy' },
        { name: 'web-02', status: 'degraded' },
      ],
    };
    render(<Artifact artifact={artifact} />);
    expect(screen.getByText('Server Status')).toBeInTheDocument();
    expect(screen.getByText('web-01')).toBeInTheDocument();
    expect(screen.getByText('healthy')).toBeInTheDocument();
    expect(screen.getByText('web-02')).toBeInTheDocument();
    expect(screen.getByText('degraded')).toBeInTheDocument();
  });

  it('renders a chart container', () => {
    const artifact: ArtifactData = {
      type: 'chart',
      chartType: 'bar',
      data: [{ name: 'Jan', value: 100 }, { name: 'Feb', value: 200 }],
    };
    render(<Artifact artifact={artifact} />);
    expect(screen.getByTestId('responsive-container')).toBeInTheDocument();
    expect(screen.getByTestId('bar-chart')).toBeInTheDocument();
  });

  it('renders a report with sections', () => {
    const artifact: ArtifactData = {
      type: 'report',
      title: 'Daily Report',
      sections: [
        { type: 'text', title: 'Summary', content: 'Everything looks good.' },
        {
          type: 'metrics',
          metrics: [{ label: 'Uptime', value: '99.9%', color: 'green' }],
        },
      ],
    };
    render(<Artifact artifact={artifact} />);
    expect(screen.getByText('Daily Report')).toBeInTheDocument();
    expect(screen.getByText('Summary')).toBeInTheDocument();
    expect(screen.getByText('Everything looks good.')).toBeInTheDocument();
    expect(screen.getByText('Uptime')).toBeInTheDocument();
  });

  it('renders empty data messages', () => {
    const artifact: ArtifactData = {
      type: 'table',
      columns: [],
      rows: [],
    };
    render(<Artifact artifact={artifact} />);
    expect(screen.getByText('No table data available.')).toBeInTheDocument();
  });

  it('renders raw JSON for unknown types', () => {
    const artifact: ArtifactData = {
      type: 'raw' as ArtifactData['type'],
      title: 'Raw Data',
    };
    render(<Artifact artifact={artifact} />);
    expect(screen.getByText('Raw Data')).toBeInTheDocument();
  });
});
