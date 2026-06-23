/**
 * Format microUSD (1/1,000,000 USD) to a human-readable dollar string.
 * For amounts < $0.01, show 6 decimal places. Otherwise 2.
 */
export function formatMicroUSD(microUSD: number): string {
  const usd = microUSD / 1_000_000;
  if (usd === 0) return '$0.00';
  const abs = Math.abs(usd);
  return `${usd < 0 ? '-' : ''}$${abs.toFixed(abs < 0.01 ? 6 : 2)}`;
}

/**
 * Format yuan fen (1/100 CNY) to a human-readable yuan string.
 */
export function formatYuan(yuanFen: number): string {
  return `¥${(yuanFen / 100).toFixed(2)}`;
}

/**
 * Format a number as plain USD (e.g., for display of token prices per 1M tokens).
 */
export function formatUSD(amount: number): string {
  return `$${amount.toFixed(amount < 0.01 ? 6 : 2)}`;
}

/**
 * Convert microUSD per token to USD per 1M tokens (common AI pricing format).
 */
export function microUSDPerTokenToUSDPer1M(microUSDPerToken: number): string {
  // microUSD/token * 1M tokens = microUSD * 1M / 1M = just the microUSD value
  // Wait — these are per-token prices. 1M tokens at this price = microUSD * 1_000_000 / 1_000_000 = input price in USD per 1M
  return `$${(microUSDPerToken * 1_000_000 / 1_000_000).toFixed(2)}`;
}
