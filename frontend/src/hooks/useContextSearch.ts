import { useState, useCallback, useRef } from 'react';
import { contextSearchApi } from '../services/api';
import type { ContextEntity, ContextEntityType } from '../types';

export function useContextSearch() {
  const [results, setResults] = useState<ContextEntity[]>([]);
  const [loading, setLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();
  const cacheRef = useRef<Map<string, ContextEntity[]>>(new Map());

  const search = useCallback((type: ContextEntityType, query: string) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    const cacheKey = `${type}:${query}`;
    const cached = cacheRef.current.get(cacheKey);
    if (cached) {
      setResults(cached);
      return;
    }

    setLoading(true);
    debounceRef.current = setTimeout(async () => {
      try {
        const entities = await contextSearchApi.search(type, query);
        cacheRef.current.set(cacheKey, entities);
        setResults(entities);
      } catch {
        setResults([]);
      } finally {
        setLoading(false);
      }
    }, 200);
  }, []);

  const clear = useCallback(() => {
    setResults([]);
    setLoading(false);
  }, []);

  return { results, loading, search, clear };
}
