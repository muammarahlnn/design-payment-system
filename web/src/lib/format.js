export const formatIDR = (n) => `Rp ${Number(n || 0).toLocaleString('id-ID')}`;

export function timeAgo(iso) {
  const seconds = Math.max(0, (Date.now() - new Date(iso)) / 1000);
  if (seconds < 60) return `${Math.floor(seconds)}s ago`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

export const clockTime = (iso) => new Date(iso).toLocaleTimeString('en-GB');

export const newIdempotencyKey = () =>
  `order-${Math.random().toString(36).slice(2, 8)}-${Date.now().toString().slice(-5)}`;
