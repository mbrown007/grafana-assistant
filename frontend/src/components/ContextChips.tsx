import { Database, LayoutDashboard, Activity, Tag, X } from 'lucide-react';
import type { ContextEntity, ContextEntityType } from '../types';

const entityIcons: Record<ContextEntityType, React.ElementType> = {
  datasource: Database,
  dashboard: LayoutDashboard,
  metric: Activity,
  label: Tag,
};

interface ContextChipsProps {
  entities: ContextEntity[];
  onRemove: (entity: ContextEntity) => void;
}

export default function ContextChips({ entities, onRemove }: ContextChipsProps) {
  if (entities.length === 0) return null;

  return (
    <div className="flex flex-wrap gap-1">
      {entities.map((entity) => {
        const Icon = entityIcons[entity.type] ?? Tag;
        return (
          <span
            key={`${entity.type}:${entity.id}`}
            className="inline-flex items-center gap-1 rounded-full bg-blue-500/15 px-2.5 py-0.5 text-xs text-blue-300 border border-blue-500/30"
          >
            <Icon className="h-3 w-3 shrink-0" />
            <span className="truncate max-w-[140px]">{entity.display_name}</span>
            <button
              type="button"
              onClick={() => onRemove(entity)}
              className="ml-0.5 rounded-full p-0.5 hover:bg-blue-500/30 transition-colors"
              aria-label={`Remove ${entity.display_name}`}
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        );
      })}
    </div>
  );
}
