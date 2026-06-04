export const appVersion = {
  name: 'dbback',
  major: 3,
  label: 'v3',
  cacheKey: 'v3-v2-weights-active-abs',
  v2_stable: true,
  stable: true,
} as const;

export type AppVersion = typeof appVersion;
