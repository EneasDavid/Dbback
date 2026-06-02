export const appVersion = {
  name: 'dbback',
  major: 2,
  label: 'v2-stable',
  cacheKey: 'v2-stable',
  v2_stable: true,
} as const;

export type AppVersion = typeof appVersion;
