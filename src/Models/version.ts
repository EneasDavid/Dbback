export const appVersion = {
  name: 'dbback',
  major: 2,
  label: 'v2',
  cacheKey: 'v2',
  v2_stable: true,
  stable: true,
} as const;

export type AppVersion = typeof appVersion;
