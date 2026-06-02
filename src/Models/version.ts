export const appVersion = {
  name: 'dbback',
  major: 3,
  label: 'v3',
  cacheKey: 'v3-abs-strict-comments',
  v2_stable: true,
  stable: true,
} as const;

export type AppVersion = typeof appVersion;
