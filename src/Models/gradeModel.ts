import type { GradeCache, GradeCard as GradeCardPayload, GradeDetail, GradeResult, GradeTable } from './types';
import { appVersion } from './version';

const GRADE_CACHE_PREFIX = 'dbback-grades:';

type GradeCacheRef = {
  current: GradeCache;
};

type LegacyColumn = {
  key?: string;
  label?: string;
  value?: string;
  comment?: string;
  commentAuthor?: string;
};

type LegacyItem = {
  key?: string;
  subtopic?: string;
  notaMaxima?: string;
  notaAlcancada?: string;
  comentario?: string;
  comentarioAutor?: string;
};

type LegacyGradeTable = GradeTable & {
  columns?: LegacyColumn[];
  items?: LegacyItem[];
};

export function storeGradeCache(
  current: GradeCache,
  incoming: GradeCache,
  cacheKey: string,
  gradesRef: GradeCacheRef,
) {
  const normalizedIncoming = normalizeGradeCache(incoming);
  const next = { ...current, ...normalizedIncoming };
  gradesStorageSet(cacheKey, next);
  gradesRef.current = next;
  return JSON.stringify(current) === JSON.stringify(next) ? current : next;
}

export function replaceGradeCache(
  current: GradeCache,
  incoming: GradeCache,
  cacheKey: string,
  gradesRef: GradeCacheRef,
) {
  const next = normalizeGradeCache(incoming);
  gradesStorageSet(cacheKey, next);
  gradesRef.current = next;
  return JSON.stringify(current) === JSON.stringify(next) ? current : next;
}

export function gradeCacheKey(matricula: string) {
  const stableSegment = appVersion.v2_stable ? appVersion.cacheKey : `v${appVersion.major}`;
  return `${GRADE_CACHE_PREFIX}${stableSegment}:${matricula}`;
}

export function readGradeCache(cacheKey: string): GradeCache {
  const cached = window.sessionStorage.getItem(cacheKey);
  if (!cached) return {};
  try {
    return normalizeGradeCache(JSON.parse(cached) as GradeCache);
  } catch {
    window.sessionStorage.removeItem(cacheKey);
    return {};
  }
}

export function normalizeGradeCache(cache: GradeCache): GradeCache {
  if (!cache || typeof cache !== 'object') return {};
  return Object.fromEntries(
    Object.entries(cache)
      .filter((entry): entry is [string, GradeResult] => Boolean(entry[0]) && Boolean(entry[1]))
      .map(([key, grade]) => [key, normalizeGradeResult(grade)]),
  );
}

export function gradeKeys(cache: GradeCache): string[] {
  return Object.entries(cache)
    .filter(([, grade]) => Boolean(grade) && grade?.active !== false)
    .sort(([, left], [, right]) => gradeOrderValue(left) - gradeOrderValue(right))
    .map(([key]) => key);
}

export function gradeLabels(cache: GradeCache): Record<string, string> {
  return Object.fromEntries(
    Object.entries(cache)
      .filter((entry): entry is [string, GradeResult] => Boolean(entry[0]) && Boolean(entry[1]) && entry[1]?.active !== false)
      .map(([key, grade]) => [key, grade.exam || key.toUpperCase()]),
  );
}

export function cardsFor(table: GradeTable) {
  return Array.isArray(table.cards) ? table.cards : [];
}

export function hasRenderableGrade(grade?: GradeResult) {
  return grade?.active !== false && Boolean(grade?.tables?.some((table) => cardsFor(table).length > 0));
}

export function isSummaryTable(kind: string) {
  return kind === 'summary' || kind === 'ab1summary' || kind === 'ab2summary';
}

export function isMediaTable(table: GradeTable) {
  return table.kind === 'ab1summary' || table.kind === 'ab2summary' || table.key.startsWith('media-');
}

export function clearGradeCache(matricula?: string) {
  for (const key of Object.keys(window.sessionStorage)) {
    if (!key.startsWith(GRADE_CACHE_PREFIX)) continue;
    if (!matricula || key.endsWith(`:${matricula}`)) {
      window.sessionStorage.removeItem(key);
    }
  }
}

export function matriculasDiffer(left: string, right: string) {
  return left.trim() !== '' && right.trim() !== '' && left.trim() !== right.trim();
}

export function isSessionExpired(error: unknown) {
  return error instanceof Error && error.message.toLowerCase().includes('sessao expirada');
}

function gradeOrderValue(grade?: GradeResult) {
  if (!grade) return Number.MAX_SAFE_INTEGER;
  const label = asText(grade.exam).toLowerCase();
  const match = label.match(/\d+/);
  return match ? Number(match[0]) : Number.MAX_SAFE_INTEGER;
}

function normalizeGradeResult(grade: GradeResult): GradeResult {
  const tables = Array.isArray(grade.tables)
    ? grade.tables
        .filter(Boolean)
        .map((table) => normalizeGradeTable(table as LegacyGradeTable))
    : [];
  return {
    ...grade,
    tables: hideAverageUntilActivitiesComplete(tables),
  };
}

function normalizeGradeTable(table: LegacyGradeTable): GradeTable {
  const cards = Array.isArray(table.cards) && table.cards.length > 0 ? table.cards : legacyCards(table);
  return {
    ...table,
    cards: cards
      .filter(Boolean)
      .map((card, index) => normalizeGradeCard(card, index))
      .filter((card) => !isPendingAverageCard(table, card))
      .filter((card) => !isSummaryTable(table.kind) || isMediaTable(table) || isVisibleSummaryCard(card)),
  };
}

function normalizeGradeCard(card: GradeCardPayload, index: number): GradeCardPayload {
  const label = asText(card.label) || 'Nota';
  const value = asText(card.value);
  const display = normalizedCardDisplay(label, value, asText(card.displayValue));
  const details = normalizeGradeDetails(card.details);
  return {
    ...card,
    key: asText(card.key) || `card-${index}`,
    label,
    value,
    displayValue: display,
    tone: card.tone || scoreTone(label, value, display),
    details: details.length > 0 ? details : undefined,
  };
}

function normalizeGradeDetails(details?: GradeDetail[]) {
  if (!Array.isArray(details)) return [];
  return details.filter(Boolean).map((detail, index) => {
    const label = asText(detail.label) || `Critério ${index + 1}`;
    const value = asText(detail.value);
    const max = Number.isFinite(detail.max) ? detail.max : 0;
    const pending = Boolean(detail.pending) || isPendingValue(value);
    const obtained = parseScore(value);
    const ratio = Number.isFinite(detail.ratio)
      ? detail.ratio
      : !pending && obtained !== null && max > 0
        ? Math.min((obtained / max) * 100, 100)
        : 0;
    return {
      ...detail,
      key: asText(detail.key) || `detail-${index}`,
      label,
      value,
      max,
      displayScore: detail.percentage
        ? percentageDisplayScore(ratio, pending)
        : pending || max > 0
          ? detailDisplayScore(value, max, pending)
          : asText(detail.displayScore) || detailDisplayScore(value, max, pending),
      ratio,
      pending,
      tone: detail.tone || scoreToneFromRatio(ratio, pending),
    };
  });
}

function legacyCards(table: LegacyGradeTable): GradeCardPayload[] {
  const columns = Array.isArray(table.columns) ? table.columns : [];
  const items = Array.isArray(table.items) ? table.items : [];
  if (items.length > 0) {
    const details = legacyDetails(items);
    const total = columns[0] ?? legacyTotalColumn(items);
    if (!total) return [];
    const label = asText(total.label) || 'Nota';
    const value = asText(total.value);
    const score = parseScore(value);
    return [
      {
        key: asText(total.key) || 'nota',
        label,
        value,
        displayValue: score !== null ? scoreComparisonDisplay(score, 1) : displayValue(label, value),
        tone: scoreTone(label, value, displayValue(label, value)),
        comment: total.comment,
        commentAuthor: total.commentAuthor,
        details,
      },
    ];
  }
  return columns
    .filter((column) => isVisibleLegacyColumn(column))
    .map((column, index) => {
      const label = asText(column.label) || `Nota ${index + 1}`;
      const value = asText(column.value);
      return {
        key: asText(column.key) || `column-${index}`,
        label,
        value,
        displayValue: displayValue(label, value),
        tone: scoreTone(label, value, displayValue(label, value)),
        comment: column.comment,
        commentAuthor: column.commentAuthor,
      };
    });
}

function legacyDetails(items: LegacyItem[]): GradeDetail[] {
  return items
    .filter((item) => normalized(asText(item.subtopic)) !== 'total')
    .map((item, index) => {
      const value = asText(item.notaAlcancada);
      const max = parseScore(asText(item.notaMaxima)) ?? 0;
      const obtained = parseScore(value);
      const pending = value.trim() === '';
      const ratio = !pending && obtained !== null && max > 0 ? Math.min((obtained / max) * 100, 100) : 0;
      return {
        key: asText(item.key) || `item-${index}`,
        label: asText(item.subtopic).trim(),
        value,
        max,
        displayScore: detailDisplayScore(value, max, pending),
        ratio,
        pending,
        tone: scoreToneFromRatio(ratio, pending),
        comment: item.comentario,
        commentAuthor: item.comentarioAutor,
      };
    });
}

function legacyTotalColumn(items: LegacyItem[]): LegacyColumn | null {
  const total = items.find((item) => normalized(asText(item.subtopic)) === 'total');
  if (!total) return null;
  const value = activityScore(asText(total.notaAlcancada), asText(total.notaMaxima));
  return { key: 'nota', label: 'Nota', value, comment: total.comentario, commentAuthor: total.comentarioAutor };
}

function isPendingAverageCard(table: GradeTable, card: { label?: string; displayValue?: string; value?: string }) {
  if (!isSummaryTable(table.kind)) return false;
  const label = (card.label || '').toLowerCase();
  const value = `${card.displayValue || card.value || ''}`.toLowerCase();
  return (label.includes('média') || label.includes('media')) && (value.includes('não corrigida') || value.includes('em correção') || value.includes('em correcao'));
}

function isVisibleSummaryCard(card: { label?: string }) {
  const label = normalized(card.label || '');
  return label.includes('prova') || label.includes('media') || label.includes('somatorio');
}

function isVisibleLegacyColumn(column: LegacyColumn) {
  const label = normalized(asText(column.label));
  return label !== '' &&
    label !== 'grupo' &&
    label !== 'equipe' &&
    !label.includes('matricula') &&
    !label.includes('nome do aluno') &&
    label !== 'nome' &&
    label !== 'aluno' &&
    (asText(column.value).trim() !== '' || Boolean(column.comment?.trim()));
}

function displayValue(label: string, value: string) {
  if (isPendingValue(value)) return 'Em correção';
  if (isGradeLabel(label)) {
    const score = parseScore(value);
    if (score === null) return 'Em correção';
    return formatScore(score);
  }
  return value;
}

function normalizedCardDisplay(label: string, value: string, display: string) {
  if (isPendingValue(value)) return 'Em correção';
  const comparison = parseDisplayScore(display);
  if (comparison) return scoreComparisonDisplay(comparison.value, comparison.max);
  return isGradeLabel(label) ? displayValue(label, value) : display || displayValue(label, value);
}

function detailDisplayScore(value: string, max: number, pending: boolean) {
  if (pending) return 'Em correção';
  const obtained = parseScore(value);
  if (obtained !== null && max > 0) return scoreComparisonDisplay(obtained, max);
  if (max > 0) return `Max ${formatScore(max)}`;
  return value.trim();
}

function percentageDisplayScore(ratio: number, pending: boolean) {
  if (pending) return 'Em correção';
  const percentage = Math.min(Math.max(Number.isFinite(ratio) ? ratio : 0, 0), 100);
  return `${percentage.toLocaleString('pt-BR', { maximumFractionDigits: 2 })}%`;
}

function scoreTone(label: string, value: string, display = '') {
  const displayScore = parseDisplayScore(display);
  if (displayScore && displayScore.max > 0) {
    return scoreToneFromRatio(Math.min((displayScore.value / displayScore.max) * 100, 100), false);
  }
  const score = parseScore(value);
  if (score === null) return isPendingValue(value) && isGradeLabel(label) ? 'score-pending' : '';
  if (!isGradeLabel(label)) return '';
  if (score <= 1) return scoreToneFromRatio(score * 100, false);
  return scoreToneFromRatio((score / 10) * 100, false);
}

function scoreToneFromRatio(ratio: number, pending: boolean) {
  if (pending) return 'score-pending';
  if (ratio <= 30) return 'score-danger';
  if (ratio < 70) return 'score-warning';
  return 'score-success';
}

function parseDisplayScore(value: string) {
  const parts = value.split(/\s*(?:\/|de)\s*/i);
  if (parts.length !== 2) return null;
  const [left, right] = parts.map((part) => parseScore(part));
  if (left === null || right === null) return null;
  return { value: left, max: right };
}

function activityScore(value: string, maximum: string) {
  const score = parseScore(value);
  const maxScore = parseScore(maximum);
  if (score === null || maxScore === null || maxScore === 0) return value;
  return formatScore(maxScore === 10 ? score / 10 : score / maxScore);
}

function isPendingValue(value: string) {
  const text = normalized(value);
  return text === '' || text.includes('nao corrigid') || text.includes('em correcao');
}

function isGradeLabel(label: string) {
  const value = normalized(label);
  return value === 'nota' ||
    value.includes('prova') ||
    value.includes('nota ab') ||
    value === 'total' ||
    value.includes('media') ||
    value.includes('projeto') ||
    value.startsWith('at.') ||
    value.startsWith('at ') ||
    value.includes('atividade');
}

function parseScore(value: string) {
  const parsed = Number(value.replace(',', '.').replace(/[^\d.-]/g, ''));
  return Number.isFinite(parsed) ? parsed : null;
}

function formatScore(value: number) {
  return value.toLocaleString('pt-BR', { maximumFractionDigits: 2, minimumFractionDigits: 2 });
}

function scoreComparisonDisplay(value: number, max: number) {
  return `${formatScore(value)} de ${formatScore(max)}`;
}

function normalized(value: string) {
  return value
    .toLowerCase()
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .trim();
}

function asText(value: unknown) {
  return typeof value === 'string' ? value : '';
}

function hideAverageUntilActivitiesComplete(tables: GradeTable[]) {
  const hasIncompleteActivity = tables.some((table) => scoredActivityTable(table) && !activityTableComplete(table));
  if (!hasIncompleteActivity) return tables;
  return tables
    .filter((table) => !isMediaTable(table))
    .map((table) => ({
      ...table,
      cards: cardsFor(table).filter((card) => !normalized(card.label).includes('media')),
    }));
}

function scoredActivityTable(table: GradeTable) {
  return !table.scoreless && (table.kind === 'activity' || table.kind === 'project');
}

function activityTableComplete(table: GradeTable) {
  const status = normalized(table.status || '');
  if (status) return status === 'encerrado';
  const cards = cardsFor(table);
  return Boolean(table.complete) &&
    cards.length > 0 &&
    cards.every((card) =>
      !isPendingValue(card.value) &&
      (card.details || []).every((detail) => !detail.pending && !isPendingValue(detail.value)),
    );
}

function gradesStorageSet(cacheKey: string, grades: GradeCache) {
  window.sessionStorage.setItem(cacheKey, JSON.stringify(grades));
}
