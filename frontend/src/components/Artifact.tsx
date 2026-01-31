import React, { useMemo } from 'react';
import {
  Bar,
  BarChart,
  Cell,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  AreaChart,
  Area,
} from 'recharts';
import {
  Activity,
  AlertCircle,
  CheckCircle,
  Clock,
  MessageSquare,
  Phone,
  Server,
  TrendingDown,
  TrendingUp,
  Users,
} from 'lucide-react';

export interface ArtifactData {
  type: 'report' | 'chart' | 'table' | 'metric-cards' | 'raw';
  title?: string;
  subtitle?: string;
  description?: string;
  chartType?: 'bar' | 'line' | 'pie' | 'area';
  data?: Array<Record<string, unknown>>;
  metrics?: MetricCard[];
  columns?: TableColumn[];
  rows?: Array<Record<string, unknown>>;
  sections?: ReportSection[];
}

interface MetricCard {
  label: string;
  value: string | number;
  change?: number;
  changeLabel?: string;
  icon?: string;
  color?: 'blue' | 'green' | 'red' | 'amber' | 'purple';
}

interface TableColumn {
  key: string;
  label: string;
  align?: 'left' | 'center' | 'right';
}

interface ReportSection {
  type: 'header' | 'summary' | 'metrics' | 'chart' | 'table' | 'text';
  title?: string;
  content?: string;
  data?: Array<Record<string, unknown>>;
  chartType?: 'bar' | 'line' | 'pie' | 'area';
  metrics?: MetricCard[];
  columns?: TableColumn[];
  rows?: Array<Record<string, unknown>>;
}

const CHART_COLORS = ['#38bdf8', '#34d399', '#f59e0b', '#fb7185', '#a78bfa', '#22d3ee'];

const ICON_MAP: Record<string, React.ElementType> = {
  users: Users,
  activity: Activity,
  alert: AlertCircle,
  success: CheckCircle,
  clock: Clock,
  server: Server,
  phone: Phone,
  message: MessageSquare,
  trending_up: TrendingUp,
  trending_down: TrendingDown,
};

export function parseArtifacts(content: string): { artifacts: ArtifactData[]; remainingContent: string } {
  const artifacts: ArtifactData[] = [];
  let remaining = content;
  const artifactRegex = /```artifact\s*([\s\S]*?)```/g;
  let match: RegExpExecArray | null = null;

  while ((match = artifactRegex.exec(content)) !== null) {
    try {
      const artifactJson = match[1].trim();
      const artifact = JSON.parse(artifactJson) as ArtifactData;
      artifacts.push(artifact);
      remaining = remaining.replace(match[0], '').trim();
    } catch (error) {
      console.error('Failed to parse artifact', error);
    }
  }

  return { artifacts, remainingContent: remaining };
}

function MetricCardTile({ metric }: { metric: MetricCard }) {
  const Icon = metric.icon && ICON_MAP[metric.icon] ? ICON_MAP[metric.icon] : Activity;
  const colorClass = metric.color ? `metric-${metric.color}` : 'metric-blue';

  return (
    <div className={`metric-card ${colorClass}`}>
      <div>
        <div className="metric-label">{metric.label}</div>
        <div className="metric-value">{metric.value}</div>
        {metric.change !== undefined && (
          <div className="metric-change">
            {metric.change > 0 ? <TrendingUp size={14} /> : metric.change < 0 ? <TrendingDown size={14} /> : null}
            <span>{metric.change > 0 ? '+' : ''}{metric.change}%</span>
            {metric.changeLabel && <span className="metric-change-label">{metric.changeLabel}</span>}
          </div>
        )}
      </div>
      <Icon size={28} />
    </div>
  );
}

function ChartBlock({ data = [], chartType = 'bar' }: { data?: Array<Record<string, unknown>>; chartType?: 'bar' | 'line' | 'pie' | 'area' }) {
  if (!data || data.length === 0) {
    return <div className="artifact-empty">No chart data available.</div>;
  }

  const keys = Object.keys(data[0] || {}).filter((key) => key !== 'name' && key !== 'label' && typeof data[0][key] === 'number');

  if (chartType === 'pie') {
    return (
      <ResponsiveContainer width="100%" height={280}>
        <PieChart>
          <Pie data={data} dataKey={keys[0] || 'value'} nameKey="name" outerRadius={100} label>
            {data.map((_, idx) => (
              <Cell key={`cell-${idx}`} fill={CHART_COLORS[idx % CHART_COLORS.length]} />
            ))}
          </Pie>
          <Tooltip />
          <Legend />
        </PieChart>
      </ResponsiveContainer>
    );
  }

  if (chartType === 'line') {
    return (
      <ResponsiveContainer width="100%" height={280}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis dataKey="name" stroke="#94a3b8" />
          <YAxis stroke="#94a3b8" />
          <Tooltip />
          <Legend />
          {keys.map((key, idx) => (
            <Line key={key} type="monotone" dataKey={key} stroke={CHART_COLORS[idx % CHART_COLORS.length]} strokeWidth={2} />
          ))}
        </LineChart>
      </ResponsiveContainer>
    );
  }

  if (chartType === 'area') {
    return (
      <ResponsiveContainer width="100%" height={280}>
        <AreaChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis dataKey="name" stroke="#94a3b8" />
          <YAxis stroke="#94a3b8" />
          <Tooltip />
          <Legend />
          {keys.map((key, idx) => (
            <Area key={key} type="monotone" dataKey={key} stroke={CHART_COLORS[idx % CHART_COLORS.length]} fill={CHART_COLORS[idx % CHART_COLORS.length]} fillOpacity={0.2} />
          ))}
        </AreaChart>
      </ResponsiveContainer>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={280}>
      <BarChart data={data}>
        <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
        <XAxis dataKey="name" stroke="#94a3b8" />
        <YAxis stroke="#94a3b8" />
        <Tooltip />
        <Legend />
        {keys.map((key, idx) => (
          <Bar key={key} dataKey={key} fill={CHART_COLORS[idx % CHART_COLORS.length]} />
        ))}
      </BarChart>
    </ResponsiveContainer>
  );
}

function TableBlock({ columns = [], rows = [] }: { columns?: TableColumn[]; rows?: Array<Record<string, unknown>> }) {
  if (!rows.length || !columns.length) {
    return <div className="artifact-empty">No table data available.</div>;
  }

  return (
    <div className="artifact-table">
      <table>
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column.key} style={{ textAlign: column.align ?? 'left' }}>
                {column.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, idx) => (
            <tr key={`row-${idx}`}>
              {columns.map((column) => (
                <td key={`${column.key}-${idx}`} style={{ textAlign: column.align ?? 'left' }}>
                  {String(row[column.key] ?? '')}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ReportSectionBlock({ section }: { section: ReportSection }) {
  if (section.type === 'metrics' && section.metrics) {
    return (
      <div className="metric-grid">
        {section.metrics.map((metric, idx) => (
          <MetricCardTile key={`${metric.label}-${idx}`} metric={metric} />
        ))}
      </div>
    );
  }

  if (section.type === 'chart' && section.data) {
    return <ChartBlock data={section.data} chartType={section.chartType} />;
  }

  if (section.type === 'table' && section.columns && section.rows) {
    return <TableBlock columns={section.columns} rows={section.rows} />;
  }

  return <p className="artifact-text">{section.content}</p>;
}

export function Artifact({ artifact }: { artifact: ArtifactData }) {
  const content = useMemo(() => {
    if (artifact.type === 'chart') {
      return <ChartBlock data={artifact.data} chartType={artifact.chartType} />;
    }
    if (artifact.type === 'table') {
      return <TableBlock columns={artifact.columns} rows={artifact.rows} />;
    }
    if (artifact.type === 'metric-cards') {
      return (
        <div className="metric-grid">
          {artifact.metrics?.map((metric, idx) => (
            <MetricCardTile key={`${metric.label}-${idx}`} metric={metric} />
          ))}
        </div>
      );
    }
    if (artifact.type === 'report') {
      return (
        <div className="artifact-report">
          {artifact.sections?.map((section, idx) => (
            <div key={`${section.type}-${idx}`} className="artifact-section">
              {section.title && <div className="artifact-section-title">{section.title}</div>}
              <ReportSectionBlock section={section} />
            </div>
          ))}
        </div>
      );
    }

    return (
      <pre className="artifact-raw">
        {JSON.stringify(artifact, null, 2)}
      </pre>
    );
  }, [artifact]);

  return (
    <div className="artifact-card">
      {(artifact.title || artifact.subtitle || artifact.description) && (
        <div className="artifact-header">
          {artifact.title && <div className="artifact-title">{artifact.title}</div>}
          {artifact.subtitle && <div className="artifact-subtitle">{artifact.subtitle}</div>}
          {artifact.description && <div className="artifact-description">{artifact.description}</div>}
        </div>
      )}
      {content}
    </div>
  );
}
