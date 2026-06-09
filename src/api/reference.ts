import { yfRequest } from './client';

/**
 * Yahoo ticker lookup. The Go plugin backend handles this natively via
 * `github.com/wnjoon/go-yfinance/pkg/lookup`; no Python service in the
 * loop after Phase 5.
 */

export type LookupCandidate = {
  symbol: string;
  short_name: string | null;
  exchange: string | null;
  type: string | null;
};

export function lookupSymbol(query: string, limit = 10) {
  return yfRequest<LookupCandidate[]>(
    `/lookup?q=${encodeURIComponent(query)}&limit=${limit}`,
  );
}
