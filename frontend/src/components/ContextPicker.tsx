import { useEffect, useRef, useState, useCallback } from 'react';
import { Database, LayoutDashboard, Activity, Tag, Loader2 } from 'lucide-react';
import { useContextSearch } from '../hooks/useContextSearch';
import type { ContextEntity, ContextEntityType } from '../types';

const ENTITY_TABS: { type: ContextEntityType; label: string; Icon: React.ElementType }[] = [
  { type: 'datasource', label: 'Datasources', Icon: Database },
  { type: 'dashboard', label: 'Dashboards', Icon: LayoutDashboard },
  { type: 'metric', label: 'Metrics', Icon: Activity },
  { type: 'label', label: 'Labels', Icon: Tag },
];

interface ContextPickerProps {
  open: boolean;
  anchorRect: { top: number; left: number } | null;
  onSelect: (entity: ContextEntity) => void;
  onClose: () => void;
}

export default function ContextPicker({ open, anchorRect, onSelect, onClose }: ContextPickerProps) {
  const [activeTab, setActiveTab] = useState<ContextEntityType>('datasource');
  const [query, setQuery] = useState('');
  const [highlightIndex, setHighlightIndex] = useState(0);
  const { results, loading, search, clear } = useContextSearch();
  const searchInputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Focus search input on open
  useEffect(() => {
    if (open) {
      setTimeout(() => searchInputRef.current?.focus(), 50);
    }
  }, [open]);

  // Load initial results when tab changes
  useEffect(() => {
    if (open) {
      search(activeTab, query);
      setHighlightIndex(0);
    }
  }, [activeTab, open, search, query]);

  // Close on click outside
  useEffect(() => {
    if (!open) return;
    const handleClick = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [open, onClose]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        setHighlightIndex((prev) => Math.min(prev + 1, results.length - 1));
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setHighlightIndex((prev) => Math.max(prev - 1, 0));
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (results[highlightIndex]) {
          onSelect(results[highlightIndex]);
        }
      }
    },
    [results, highlightIndex, onSelect, onClose],
  );

  const handleTabChange = useCallback(
    (type: ContextEntityType) => {
      setActiveTab(type);
      setQuery('');
      clear();
      setHighlightIndex(0);
    },
    [clear],
  );

  if (!open) return null;

  const style: React.CSSProperties = anchorRect
    ? { position: 'fixed', bottom: `calc(100vh - ${anchorRect.top}px + 8px)`, left: anchorRect.left, zIndex: 60 }
    : { position: 'absolute', bottom: '100%', left: 0, zIndex: 60 };

  return (
    <div ref={containerRef} style={style} className="w-80 rounded-lg border border-border bg-card shadow-xl" onKeyDown={handleKeyDown}>
      {/* Category tabs */}
      <div className="flex border-b border-border">
        {ENTITY_TABS.map(({ type, label, Icon }) => (
          <button
            key={type}
            type="button"
            onClick={() => handleTabChange(type)}
            className={`flex-1 flex items-center justify-center gap-1 px-2 py-2 text-xs transition-colors ${
              activeTab === type ? 'text-blue-400 border-b-2 border-blue-400' : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <Icon className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">{label}</span>
          </button>
        ))}
      </div>

      {/* Search input */}
      <div className="p-2 border-b border-border">
        <input
          ref={searchInputRef}
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={`Search ${ENTITY_TABS.find((t) => t.type === activeTab)?.label?.toLowerCase() ?? ''}...`}
          className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
      </div>

      {/* Results */}
      <div className="max-h-48 overflow-y-auto p-1">
        {loading && (
          <div className="flex items-center justify-center py-4 text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span className="ml-2 text-xs">Searching...</span>
          </div>
        )}
        {!loading && results.length === 0 && (
          <div className="py-4 text-center text-xs text-muted-foreground">
            {query ? 'No results found' : 'Type to search or browse'}
          </div>
        )}
        {!loading &&
          results.map((entity, index) => {
            const TabIcon = ENTITY_TABS.find((t) => t.type === entity.type)?.Icon ?? Tag;
            const subtitle =
              entity.type === 'datasource'
                ? entity.metadata?.ds_type
                : entity.type === 'dashboard'
                  ? entity.metadata?.folder
                  : undefined;
            return (
              <button
                key={`${entity.type}:${entity.id}`}
                type="button"
                onClick={() => onSelect(entity)}
                className={`w-full flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors ${
                  index === highlightIndex ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50'
                }`}
              >
                <TabIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="truncate">{entity.display_name}</div>
                  {subtitle && <div className="truncate text-xs text-muted-foreground">{subtitle}</div>}
                </div>
              </button>
            );
          })}
      </div>
    </div>
  );
}
