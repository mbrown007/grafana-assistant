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
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';

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
  const colorClass =
    metric.color === 'green'
      ? 'border-l-green-500'
      : metric.color === 'red'
        ? 'border-l-red-500'
        : metric.color === 'amber'
          ? 'border-l-amber-500'
          : metric.color === 'purple'
            ? 'border-l-purple-500'
            : 'border-l-blue-500';

  return (
    <Card className={`border-l-4 ${colorClass}`}>
      <CardContent className="flex items-center justify-between p-3">
        <div>
          <div className="text-xs text-muted-foreground">{metric.label}</div>
          <div className="text-xl font-semibold mt-1">{metric.value}</div>
          {metric.change !== undefined && (
            <div className="flex items-center gap-1 text-xs text-muted-foreground mt-1">
              {metric.change > 0 ? <TrendingUp size={14} /> : metric.change < 0 ? <TrendingDown size={14} /> : null}
              <span>{metric.change > 0 ? '+' : ''}{metric.change}%</span>
              {metric.changeLabel && <span className="opacity-70">{metric.changeLabel}</span>}
            </div>
          )}
        </div>
        <Icon size={28} />
      </CardContent>
    </Card>
  );
}

function ChartBlock({ data = [], chartType = 'bar' }: { data?: Array<Record<string, unknown>>; chartType?: 'bar' | 'line' | 'pie' | 'area' }) {
  if (!data || data.length === 0) {
    return <div className="text-center text-muted-foreground py-4">No chart data available.</div>;
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
    return <div className="text-center text-muted-foreground py-4">No table data available.</div>;
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          {columns.map((column) => (
            <TableHead key={column.key} style={{ textAlign: column.align ?? 'left' }}>
              {column.label}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((row, idx) => (
          <TableRow key={`row-${idx}`}>
            {columns.map((column) => (
              <TableCell key={`${column.key}-${idx}`} style={{ textAlign: column.align ?? 'left' }}>
                {String(row[column.key] ?? '')}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

function ReportSectionBlock({ section }: { section: ReportSection }) {
  if (section.type === 'metrics' && section.metrics) {
    return (
        <div className="grid grid-cols-[repeat(auto-fit,minmax(160px,1fr))] gap-3">
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

  return <p className="m-0 text-muted-foreground">{section.content}</p>;
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
        <div className="grid grid-cols-[repeat(auto-fit,minmax(160px,1fr))] gap-3">
          {artifact.metrics?.map((metric, idx) => (
            <MetricCardTile key={`${metric.label}-${idx}`} metric={metric} />
          ))}
        </div>
      );
    }
    if (artifact.type === 'report') {
      return (
        <div className="grid gap-4">
          {artifact.sections?.map((section, idx) => (
            <div key={`${section.type}-${idx}`}>
              {section.title && <div className="font-semibold mb-2">{section.title}</div>}
              <ReportSectionBlock section={section} />
            </div>
          ))}
        </div>
      );
    }

    return (
      <pre className="whitespace-pre-wrap bg-muted rounded-lg p-3 text-muted-foreground">
        {JSON.stringify(artifact, null, 2)}
      </pre>
    );
  }, [artifact]);

  return (
    <Card>
      {(artifact.title || artifact.subtitle || artifact.description) && (
        <CardHeader>
          {artifact.title && <CardTitle>{artifact.title}</CardTitle>}
          {artifact.subtitle && <CardDescription>{artifact.subtitle}</CardDescription>}
          {artifact.description && (
            <div className="text-xs text-muted-foreground mt-1">{artifact.description}</div>
          )}
        </CardHeader>
      )}
      <CardContent className={artifact.title || artifact.subtitle || artifact.description ? '' : 'pt-4'}>
        {content}
      </CardContent>
    </Card>
  );
}
