export const appVersion = {
  name: 'dbback',
  major: 3,
  label: 'v3',
  cacheKey: 'v6-scoreless-dropdown',
  v2_stable: true,
  stable: true,
} as const;

export type AppVersion = typeof appVersion;
