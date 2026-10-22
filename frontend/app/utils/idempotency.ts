// createIdempotencyKey returns a unique key for retry-safe API mutations.
export function createIdempotencyKey() {
  return globalThis.crypto.randomUUID()
}
