import { StrictMode, useEffect, useMemo, useRef, useState } from 'react';
import type { FormEvent, MutableRefObject } from 'react';
import { createRoot } from 'react-dom/client';
import { api } from './api';
import { EmptyState, ExamSwitch, GradeCard, InlineError, LoginView, SummaryTable, Topbar } from './components';
import type { GradeCache, GradeResult, GradeTable, SessionUser } from './types';
import './styles.scss';

const CACHE_VERSION = 'v3';
const EMPTY_STATE_MS = 5_000;

function App() {
  const [matricula, setMatricula] = useState('');
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
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', theme === 'dark' ? '#020617' : '#0f172a');
    window.localStorage.setItem('theme', theme);
  }, [theme]);

  useEffect(() => {
    api<SessionUser>('/api/me')
      .then(setSession)
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
        const result = normalizeGradeResult(await api<GradeResult>(`/api/grades?exam=${target}`));
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
  const summaryTables = useMemo(() => visibleTables.filter((table) => isSummaryTable(table.kind) && cardsFor(table).length > 0), [visibleTables]);
  const hasRenderableTables = activityTables.length + summaryTables.length > 0;

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
    setLoading(true);
    setError('');
    try {
      const result = await api<SessionUser>('/api/login', {
        method: 'POST',
        body: JSON.stringify({ matricula }),
      });
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
      <a className="skip-link" href="#grades">Ir para notas</a>
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
          .map((table) => ({
            ...table,
            cards: Array.isArray(table.cards) ? table.cards.filter(Boolean).filter((card) => !isPendingAverageCard(table, card)) : [],
          }))
      : [],
  };
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
  return kind === 'summary' || kind === 'ab2summary';
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
