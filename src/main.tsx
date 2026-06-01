import { StrictMode, useEffect, useMemo, useRef, useState } from 'react';
import type { FormEvent, MutableRefObject } from 'react';
import { createRoot } from 'react-dom/client';
import { api } from './api';
import { EmptyState, ExamSwitch, GradeCard, InlineError, LoginView, SummaryTable, Topbar } from './components';
import type { GradeCache, GradeCard as GradeCardPayload, GradeDetail, GradeResult, GradeTable, SessionUser } from './types';
import './styles.scss';

const CACHE_VERSION = 'v16';
const EMPTY_STATE_MS = 5_000;
const LAST_MATRICULA_KEY = 'dbback-last-matricula';
const THEME_QUERY = '(prefers-color-scheme: dark)';

type Theme = 'light' | 'dark';
type Exam = string;

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

function systemTheme(): Theme {
  return window.matchMedia?.(THEME_QUERY).matches ? 'dark' : 'light';
}

function initialTheme(): Theme {
  return systemTheme();
}

function App() {
  const [matricula, setMatricula] = useState(() => window.localStorage.getItem(LAST_MATRICULA_KEY) || '');
  const [session, setSession] = useState<SessionUser | null>(null);
  const [exam, setExam] = useState<Exam>('ab1');
  const [examOrder, setExamOrder] = useState<Exam[]>([]);
  const [theme, setTheme] = useState<Theme>(() => initialTheme());
  const [grades, setGrades] = useState<GradeCache>({});
  const gradesRef = useRef<GradeCache>({});
  const [activeDetail, setActiveDetail] = useState<{ tableKey: string; cardKey: string } | null>(null);
  const [loading, setLoading] = useState(false);
  const [gradesRefreshing, setGradesRefreshing] = useState(false);
  const [showEmptyState, setShowEmptyState] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    gradesRef.current = grades;
  }, [grades]);

  useEffect(() => {
    if (!error) return;
    const timeout = window.setTimeout(() => setError(''), EMPTY_STATE_MS);
    return () => window.clearTimeout(timeout);
  }, [error]);

  useEffect(() => {
    document.documentElement.dataset.screen = session ? 'app' : 'login';
    return () => {
      delete document.documentElement.dataset.screen;
    };
  }, [session]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.querySelectorAll('meta[name="theme-color"]').forEach((meta) => {
      meta.setAttribute('content', theme === 'dark' ? '#07111f' : '#eef2f8');
    });
  }, [theme]);

  useEffect(() => {
    const media = window.matchMedia?.(THEME_QUERY);
    if (!media) return;

    const syncWithSystem = () => setTheme(media.matches ? 'dark' : 'light');
    syncWithSystem();

    if (media.addEventListener) {
      media.addEventListener('change', syncWithSystem);
      return () => media.removeEventListener('change', syncWithSystem);
    }
    media.addListener(syncWithSystem);
    return () => media.removeListener(syncWithSystem);
  }, []);

  useEffect(() => {
    api<SessionUser | null>('/api/me')
      .then((user) => {
        if (user?.matricula) {
          setSession(user);
          return;
        }
        clearClientSession();
        setSession(null);
      })
      .catch(() => {
        clearClientSession();
        setSession(null);
      });
  }, []);

  useEffect(() => {
    if (!session) return;
    const cacheKey = gradeCacheKey(session.matricula);
    const cached = readGradeCache(cacheKey);
    if (Object.keys(cached).length > 0) {
      setGrades((current) => storeGradeCache(current, cached, cacheKey, gradesRef));
    }
  }, [session]);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    const currentSession = session;
    const cacheKey = gradeCacheKey(currentSession.matricula);
    const cached = readGradeCache(cacheKey);
    const cachedGrades = { ...cached, ...gradesRef.current };
    const hasCachedVisibleGrade = hasRenderableGrade(cachedGrades[exam]);

    async function fetchVisibleGrade() {
      setGradesRefreshing(true);
      setLoading(!hasCachedVisibleGrade);
      setError('');
      try {
        const allResults = normalizeGradeCache(await api<GradeCache>('/api/grades/all?refresh=1'));
        if (cancelled) return;
        const keys = gradeKeys(allResults);
        setExamOrder(keys);
        setGrades((current) => replaceGradeCache(current, allResults, cacheKey, gradesRef));
        if (keys.length > 0 && !keys.includes(exam)) {
          setExam(keys[0]);
        }
      } catch (err) {
        if (cancelled) return;
        if (isSessionExpired(err)) {
          clearClientSession(currentSession.matricula);
          setGrades({});
          setError('');
          setSession(null);
          return;
        }
        if (!hasCachedVisibleGrade) {
          setError(err instanceof Error ? err.message : 'Erro ao carregar as notas.');
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
          setGradesRefreshing(false);
        }
      }
    }

    void fetchVisibleGrade();

    return () => {
      cancelled = true;
    };
  }, [session, exam]);

  const visibleTables = useMemo(() => grades[exam]?.tables ?? [], [grades, exam]);
  const availableExams = useMemo(() => {
    const keys = examOrder.filter((key) => grades[key]);
    if (keys.length > 0) return keys;
    return gradeKeys(grades);
  }, [examOrder, grades]);
  const examLabels = useMemo(() => gradeLabels(grades), [grades]);
  const useExamCarousel = useMemo(
    () => availableExams.length > 1 && availableExams.some((key) => grades[key]?.schemaStatus === 'v2'),
    [availableExams, grades],
  );

  const activityTables = useMemo(() => visibleTables.filter((table) => !isSummaryTable(table.kind) && cardsFor(table).length > 0), [visibleTables]);
  const summaryTables = useMemo(() => visibleTables.filter((table) => isSummaryTable(table.kind) && !isMediaTable(table) && cardsFor(table).length > 0), [visibleTables]);
  const mediaTables = useMemo(() => visibleTables.filter((table) => isMediaTable(table) && cardsFor(table).length > 0), [visibleTables]);
  const hasRenderableTables = activityTables.length + summaryTables.length + mediaTables.length > 0;

  useEffect(() => {
    if (!session || loading || hasRenderableTables) {
      setShowEmptyState(false);
      return;
    }
    setShowEmptyState(true);
    const timeout = window.setTimeout(() => setShowEmptyState(false), EMPTY_STATE_MS);
    return () => window.clearTimeout(timeout);
  }, [session, loading, hasRenderableTables, exam]);

  const handleToggleDetail = (tableKey: string, cardKey: string) => {
    setActiveDetail((current) =>
      current?.tableKey === tableKey && current.cardKey === cardKey ? null : { tableKey, cardKey }
    );
  };

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalizedMatricula = matricula.trim();
    setLoading(true);
    setError('');
    try {
      const result = await api<SessionUser>('/api/login', {
        method: 'POST',
        body: JSON.stringify({ matricula: normalizedMatricula }),
      });
      const previousMatricula = window.localStorage.getItem(LAST_MATRICULA_KEY) || '';
      const resolvedMatricula = result.matricula || normalizedMatricula;
      if (matriculasDiffer(previousMatricula, resolvedMatricula)) {
        clearClientSession();
      }
      window.localStorage.setItem(LAST_MATRICULA_KEY, resolvedMatricula);
      gradesRef.current = {};
      setGrades({});
      setExamOrder([]);
      setActiveDetail(null);
      setExam('ab1');
      setSession(result);
      setMatricula('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Erro ao entrar.');
    } finally {
      setLoading(false);
    }
  }

  async function handleLogout() {
    await api('/api/logout', { method: 'POST' }).catch(() => null);
    clearClientSession(session?.matricula);
    setSession(null);
    setGrades({});
    setExamOrder([]);
    setError('');
  }

  function handleThemeChange(nextTheme: Theme) {
    setTheme(nextTheme);
  }

  if (!session) {
    return (
      <LoginView
        matricula={matricula}
        setMatricula={setMatricula}
        loading={loading}
        error={error}
        theme={theme}
        setTheme={handleThemeChange}
        onSubmit={handleLogin}
      />
    );
  }

  return (
    <main className="shell">
      <Topbar session={session} theme={theme} setTheme={handleThemeChange} onLogout={handleLogout} />
      {!gradesRefreshing && <ExamSwitch exam={exam} exams={availableExams} labels={examLabels} carousel={useExamCarousel} setExam={setExam} />}

      {error && <InlineError message={error} />}
      {loading && <div className="loading" role="status" aria-live="polite">Carregando notas...</div>}

      {hasRenderableTables ? (
        <section className="grade-list" id="grades" aria-live="polite">
          {activityTables.map((table) => (
            <GradeCard table={table} key={table.key} activeDetail={activeDetail} onToggleDetail={handleToggleDetail} />
          ))}
          {summaryTables.map((table) => (
            <SummaryTable table={table} key={table.key} />
          ))}
          {mediaTables.map((table) => (
            <SummaryTable table={table} key={table.key} />
          ))}
        </section>
      ) : (
        !loading && showEmptyState && <EmptyState exam={exam} />
      )}
    </main>
  );
}

function storeGradeCache(
  current: GradeCache,
  incoming: GradeCache,
  cacheKey: string,
  gradesRef: MutableRefObject<GradeCache>,
) {
  const normalizedIncoming = normalizeGradeCache(incoming);
  const next = { ...current, ...normalizedIncoming };
  gradesStorageSet(cacheKey, next);
  gradesRef.current = next;
  return JSON.stringify(current) === JSON.stringify(next) ? current : next;
}

function replaceGradeCache(
  current: GradeCache,
  incoming: GradeCache,
  cacheKey: string,
  gradesRef: MutableRefObject<GradeCache>,
) {
  const next = normalizeGradeCache(incoming);
  gradesStorageSet(cacheKey, next);
  gradesRef.current = next;
  return JSON.stringify(current) === JSON.stringify(next) ? current : next;
}

function gradeCacheKey(matricula: string) {
  return `dbback-grades:${CACHE_VERSION}:${matricula}`;
}

function readGradeCache(cacheKey: string): GradeCache {
  const cached = window.sessionStorage.getItem(cacheKey);
  if (!cached) return {};
  try {
    return normalizeGradeCache(JSON.parse(cached) as GradeCache);
  } catch {
    window.sessionStorage.removeItem(cacheKey);
    return {};
  }
}

function normalizeGradeCache(cache: GradeCache): GradeCache {
  if (!cache || typeof cache !== 'object') return {};
  return Object.fromEntries(
    Object.entries(cache)
      .filter((entry): entry is [string, GradeResult] => Boolean(entry[0]) && Boolean(entry[1]))
      .map(([key, grade]) => [key, normalizeGradeResult(grade)]),
  );
}

function gradeKeys(cache: GradeCache): string[] {
  return Object.entries(cache)
    .filter(([, grade]) => Boolean(grade))
    .sort(([, left], [, right]) => gradeOrderValue(left) - gradeOrderValue(right))
    .map(([key]) => key);
}

function gradeLabels(cache: GradeCache): Record<string, string> {
  return Object.fromEntries(
    Object.entries(cache)
      .filter((entry): entry is [string, GradeResult] => Boolean(entry[0]) && Boolean(entry[1]))
      .map(([key, grade]) => [key, grade.exam || key.toUpperCase()]),
  );
}

function gradeOrderValue(grade?: GradeResult) {
  if (!grade) return Number.MAX_SAFE_INTEGER;
  const label = asText(grade.exam).toLowerCase();
  const match = label.match(/\d+/);
  return match ? Number(match[0]) : Number.MAX_SAFE_INTEGER;
}

function normalizeGradeResult(grade: GradeResult): GradeResult {
  return {
    ...grade,
    tables: Array.isArray(grade.tables)
      ? grade.tables
          .filter(Boolean)
          .map((table) => normalizeGradeTable(table as LegacyGradeTable))
      : [],
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
  const display = asText(card.displayValue) || displayValue(label, value);
  const details = normalizeGradeDetails(card.details);
  return {
    ...card,
    key: asText(card.key) || `card-${index}`,
    label,
    value,
    displayValue: display,
    tone: card.tone || scoreTone(label, value),
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
      displayScore: asText(detail.displayScore) || detailDisplayScore(value, max, pending),
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
        displayValue: score !== null ? `${formatScore(score)}/${formatScoreFixed(1, 2)}` : displayValue(label, value),
        tone: scoreTone(label, value),
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
        tone: scoreTone(label, value),
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

function cardsFor(table: GradeTable) {
  return Array.isArray(table.cards) ? table.cards : [];
}

function hasRenderableGrade(grade?: GradeResult) {
  return Boolean(grade?.tables?.some((table) => cardsFor(table).length > 0));
}

function isPendingAverageCard(table: GradeTable, card: { label?: string; displayValue?: string; value?: string }) {
  if (!isSummaryTable(table.kind)) return false;
  const label = (card.label || '').toLowerCase();
  const value = `${card.displayValue || card.value || ''}`.toLowerCase();
  return (label.includes('média') || label.includes('media')) && value.includes('não corrigida');
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
  if (isPendingValue(value)) return 'Não corrigida ainda';
  if (isGradeLabel(label) && parseScore(value) === null) return 'Não corrigida ainda';
  return value;
}

function detailDisplayScore(value: string, max: number, pending: boolean) {
  if (pending) return 'Não corrigido ainda';
  const obtained = parseScore(value);
  if (obtained !== null && max > 0) return `${formatScore(obtained)} / ${formatScore(max)}`;
  if (max > 0) return `Max ${formatScore(max)}`;
  return value.trim();
}

function scoreTone(label: string, value: string) {
  let score = parseScore(value);
  if (score === null) return isPendingValue(value) && isGradeLabel(label) ? 'score-pending' : '';
  if (!isGradeLabel(label)) return '';
  if (score <= 1) score *= 10;
  if (score < 5) return 'score-danger';
  if (score < 7) return 'score-warning';
  return 'score-success';
}

function scoreToneFromRatio(ratio: number, pending: boolean) {
  if (pending) return 'score-pending';
  if (ratio < 50) return 'score-danger';
  if (ratio < 70) return 'score-warning';
  return 'score-success';
}

function activityScore(value: string, maximum: string) {
  const score = parseScore(value);
  const maxScore = parseScore(maximum);
  if (score === null || maxScore === null || maxScore === 0) return value;
  return formatScore(maxScore === 10 ? score / 10 : score / maxScore);
}

function isPendingValue(value: string) {
  const text = normalized(value);
  return text === '' || text.includes('nao corrigid');
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
  return value.toLocaleString('pt-BR', { maximumFractionDigits: 2, minimumFractionDigits: value % 1 === 0 ? 0 : 1 });
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

function formatScoreFixed(value: number, fractionDigits: number) {
  return value.toLocaleString('pt-BR', { maximumFractionDigits: fractionDigits, minimumFractionDigits: fractionDigits });
}

function gradesStorageSet(cacheKey: string, grades: GradeCache) {
  window.sessionStorage.setItem(cacheKey, JSON.stringify(grades));
}

function isSummaryTable(kind: string) {
  return kind === 'summary' || kind === 'ab1summary' || kind === 'ab2summary';
}

function isMediaTable(table: GradeTable) {
  return table.kind === 'ab1summary' || table.kind === 'ab2summary' || table.key.startsWith('media-');
}

function clearClientSession(matricula?: string) {
  for (const key of Object.keys(window.sessionStorage)) {
    if (!key.startsWith('dbback-grades:')) continue;
    if (!matricula || key.endsWith(`:${matricula}`)) {
      window.sessionStorage.removeItem(key);
    }
  }
}

function matriculasDiffer(left: string, right: string) {
  return left.trim() !== '' && right.trim() !== '' && left.trim() !== right.trim();
}

function isSessionExpired(error: unknown) {
  return error instanceof Error && error.message.toLowerCase().includes('sessao expirada');
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
