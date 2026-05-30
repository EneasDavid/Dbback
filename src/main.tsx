import { StrictMode, useEffect, useMemo, useRef, useState } from 'react';
import type { FormEvent, MutableRefObject } from 'react';
import { createRoot } from 'react-dom/client';
import { api } from './api';
import { EmptyState, ExamSwitch, GradeCard, InlineError, LoginView, SummaryTable, Topbar } from './components';
import type { GradeCache, GradeResult, GradeTable, SessionUser } from './types';
import './styles.scss';

const CACHE_VERSION = 'v7';
const EMPTY_STATE_MS = 5_000;
const LAST_MATRICULA_KEY = 'dbback-last-matricula';

type LegacyColumn = {
  key: string;
  label: string;
  value: string;
  comment?: string;
  commentAuthor?: string;
};

type LegacyItem = {
  key: string;
  subtopic: string;
  notaMaxima: string;
  notaAlcancada: string;
  comentario?: string;
  comentarioAutor?: string;
};

type LegacyGradeTable = GradeTable & {
  columns?: LegacyColumn[];
  items?: LegacyItem[];
};

function App() {
  const [matricula, setMatricula] = useState(() => window.localStorage.getItem(LAST_MATRICULA_KEY) || '');
  const [session, setSession] = useState<SessionUser | null>(null);
  const [exam, setExam] = useState<'ab1' | 'ab2'>('ab1');
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    return window.localStorage.getItem('theme') === 'dark' ? 'dark' : 'light';
  });
  const [grades, setGrades] = useState<GradeCache>({});
  const gradesRef = useRef<GradeCache>({});
  const [activeDetail, setActiveDetail] = useState<{ tableKey: string; cardKey: string } | null>(null);
  const [loading, setLoading] = useState(false);
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
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', theme === 'dark' ? '#182131' : '#eef2f8');
    window.localStorage.setItem('theme', theme);
  }, [theme]);

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
    clearLegacyGradeCache(session.matricula);
    const cached = readGradeCache(cacheKey);
    if (Object.keys(cached).length > 0) {
      setGrades((current) => storeGradeCache(current, cached, cacheKey, gradesRef));
    }
  }, [session]);

  useEffect(() => {
    if (!session) return;
    const currentSession = session;
    let cancelled = false;
    const cacheKey = gradeCacheKey(currentSession.matricula);

    async function fetchExam(target: 'ab1' | 'ab2', foreground: boolean) {
      if (foreground) {
        setLoading(true);
        setError('');
      }
      try {
        const params = new URLSearchParams({ exam: target });
        if (foreground) params.set('refresh', '1');
        const result = normalizeGradeResult(await api<GradeResult>(`/api/grades?${params.toString()}`));
        if (cancelled) return;
        setGrades((current) => storeGradeCache(current, { [target]: result }, cacheKey, gradesRef));
      } catch (err) {
        if (cancelled) return;
        if (isSessionExpired(err)) {
          clearClientSession(currentSession.matricula);
          setGrades({});
          setError('');
          setSession(null);
          return;
        }
        if (foreground) {
          setError(err instanceof Error ? err.message : 'Erro ao carregar as notas.');
        }
      } finally {
        if (foreground && !cancelled) setLoading(false);
      }
    }

    const cached = readGradeCache(cacheKey);
    const hasActive = Boolean(gradesRef.current[exam] || cached[exam]);
    void fetchExam(exam, !hasActive).then(() => {
      const other = exam === 'ab1' ? 'ab2' : 'ab1';
      if (!cancelled && canPreload() && !gradesRef.current[other]) {
        void fetchExam(other, false);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [session, exam]);

  const visibleTables = useMemo(() => grades[exam]?.tables ?? [], [grades, exam]);

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
      window.localStorage.setItem(LAST_MATRICULA_KEY, result.matricula || normalizedMatricula);
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
    setError('');
  }

  if (!session) {
    return (
      <LoginView
        matricula={matricula}
        setMatricula={setMatricula}
        loading={loading}
        error={error}
        theme={theme}
        setTheme={setTheme}
        onSubmit={handleLogin}
      />
    );
  }

  return (
    <main className="shell">
      <Topbar session={session} theme={theme} setTheme={setTheme} onLogout={handleLogout} />
      <ExamSwitch exam={exam} setExam={setExam} />

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
  return {
    ...(cache.ab1 ? { ab1: normalizeGradeResult(cache.ab1) } : {}),
    ...(cache.ab2 ? { ab2: normalizeGradeResult(cache.ab2) } : {}),
  };
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
      .filter((card) => !isPendingAverageCard(table, card))
      .filter((card) => !isSummaryTable(table.kind) || isMediaTable(table) || isVisibleSummaryCard(card)),
  };
}

function legacyCards(table: LegacyGradeTable) {
  const columns = Array.isArray(table.columns) ? table.columns : [];
  const items = Array.isArray(table.items) ? table.items : [];
  if (items.length > 0) {
    const details = legacyDetails(items);
    const total = columns[0] ?? legacyTotalColumn(items);
    if (!total) return [];
    return [
      {
        key: total.key || 'nota',
        label: total.label || 'Nota',
        value: total.value || '',
        displayValue: displayValue(total.label || 'Nota', total.value || ''),
        tone: scoreTone(total.label || 'Nota', total.value || ''),
        comment: total.comment,
        commentAuthor: total.commentAuthor,
        details,
      },
    ];
  }
  return columns
    .filter((column) => isVisibleLegacyColumn(column))
    .map((column) => ({
      key: column.key,
      label: column.label,
      value: column.value,
      displayValue: displayValue(column.label, column.value),
      tone: scoreTone(column.label, column.value),
      comment: column.comment,
      commentAuthor: column.commentAuthor,
    }));
}

function legacyDetails(items: LegacyItem[]) {
  return items
    .filter((item) => normalized(item.subtopic) !== 'total')
    .map((item) => {
      const max = parseScore(item.notaMaxima) ?? 0;
      const obtained = parseScore(item.notaAlcancada);
      const pending = item.notaAlcancada.trim() === '';
      const ratio = !pending && obtained !== null && max > 0 ? Math.min((obtained / max) * 100, 100) : 0;
      return {
        key: item.key,
        label: humanizeLabel(item.subtopic),
        value: item.notaAlcancada,
        max,
        displayScore: detailDisplayScore(item.notaAlcancada, max, pending),
        ratio,
        pending,
        tone: scoreToneFromRatio(ratio, pending),
        comment: item.comentario,
        commentAuthor: item.comentarioAutor,
      };
    });
}

function legacyTotalColumn(items: LegacyItem[]): LegacyColumn | null {
  const total = items.find((item) => normalized(item.subtopic) === 'total');
  if (!total) return null;
  const value = activityScore(total.notaAlcancada, total.notaMaxima);
  return { key: 'nota', label: 'Nota', value, comment: total.comentario, commentAuthor: total.comentarioAutor };
}

function cardsFor(table: GradeTable) {
  return Array.isArray(table.cards) ? table.cards : [];
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
  const label = normalized(column.label);
  return label !== '' &&
    label !== 'grupo' &&
    label !== 'equipe' &&
    !label.includes('matricula') &&
    !label.includes('nome do aluno') &&
    label !== 'nome' &&
    label !== 'aluno' &&
    (column.value.trim() !== '' || Boolean(column.comment?.trim()));
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

function humanizeLabel(label: string) {
  return label
    .replace(/\b(at\.?\s*\d+)\b/i, (match) => match.toUpperCase().replace('AT', 'AT.'))
    .replace(/\bq\.?\s*(\d+)\b/i, (match) => match.toUpperCase().replace('Q', 'Q.'))
    .replace(/\bsgbd\b/i, 'SGBD')
    .replace(/\bcrud\b/i, 'CRUD')
    .replace(/\bdataset\b/i, 'Dataset')
    .replace(/\bnota\b/i, 'Nota')
    .replace(/\bm[ée]dia\b/i, 'Média')
    .replace(/\btotal\b/i, 'Total')
    .replace(/\s+/g, ' ')
    .trim();
}

function gradesStorageSet(cacheKey: string, grades: GradeCache) {
  window.sessionStorage.setItem(cacheKey, JSON.stringify(grades));
}

function canPreload() {
  const connection = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection;
  if (!connection) return true;
  if (connection.saveData) return false;
  return connection.effectiveType !== 'slow-2g' && connection.effectiveType !== '2g';
}

function isSummaryTable(kind: string) {
  return kind === 'summary' || kind === 'ab1summary' || kind === 'ab2summary';
}

function isMediaTable(table: GradeTable) {
  return table.kind === 'ab1summary' || table.kind === 'ab2summary' || table.key.startsWith('media-');
}

function clearClientSession(matricula?: string) {
  if (matricula) {
    window.sessionStorage.removeItem(gradeCacheKey(matricula));
    clearLegacyGradeCache(matricula);
  }
  if (!matricula) {
    for (const key of Object.keys(window.sessionStorage)) {
      if (key.startsWith('dbback-grades:')) window.sessionStorage.removeItem(key);
    }
  }
}

function clearLegacyGradeCache(matricula: string) {
  window.sessionStorage.removeItem(`dbback-grades:${matricula}`);
}

function isSessionExpired(error: unknown) {
  return error instanceof Error && error.message.toLowerCase().includes('sessao expirada');
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
