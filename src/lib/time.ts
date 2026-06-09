export function formatEventMicros(micros: number | null | undefined): string {
  if (micros == null) {
    return '—';
  }
  return new Date(micros / 1000).toLocaleString();
}

export function microsAgo(micros: number | null | undefined): string {
  if (!micros) {
    return '—';
  }
  const diffSec = Math.max(0, (Date.now() - micros / 1000) / 1000);
  if (diffSec < 60) {
    return `${diffSec.toFixed(0)}s ago`;
  }
  if (diffSec < 3600) {
    return `${(diffSec / 60).toFixed(0)}m ago`;
  }
  if (diffSec < 86400) {
    return `${(diffSec / 3600).toFixed(1)}h ago`;
  }
  return `${(diffSec / 86400).toFixed(1)}d ago`;
}
