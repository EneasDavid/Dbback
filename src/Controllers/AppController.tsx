import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { EmptyState, ExamSwitch, GradeCard, InlineError, LoginView, SummaryTable, Topbar } from '../Views/components';
import {
  cardsFor,
  clearGradeCache,
  gradeCacheKey,
  gradeKeys,
  gradeLabels,
  hasRenderableGrade,
  isMediaTable,
  isSessionExpired,
  isSummaryTable,
  matriculasDiffer,
  normalizeGradeCache,
  readGradeCache,
  replaceGradeCache,
  storeGradeCache,
} from '../Models/gradeModel';
import type { GradeCache, SessionUser } from '../Models/types';
import { appVersion } from '../Models/version';
import { api, apiSWR } from './apiController';

const EMPTY_STATE_MS = 5_000;
const LAST_MATRICULA_KEY = 'dbback-last-matricula';
const THEME_QUERY = '(prefers-color-scheme: dark)';

type Theme = 'light' | 'dark';
type Exam = string;

function systemTheme(): Theme {
  return window.matchMedia?.(THEME_QUERY).matches ? 'dark' : 'light';
}

function initialTheme(): Theme {
  return systemTheme();
}

export default function AppController() {
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
    document.documentElement.dataset.version = appVersion.label;
    document.documentElement.dataset.v2Stable = appVersion.v2_stable ? 'true' : 'false';
    return () => {
      delete document.documentElement.dataset.screen;
      delete document.documentElement.dataset.version;
      delete document.documentElement.dataset.v2Stable;
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
        clearGradeCache();
        setSession(null);
      })
      .catch(() => {
        clearGradeCache();
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
    const hasCachedVisibleGrade = Object.values(cachedGrades).some((grade) => hasRenderableGrade(grade));
    const controller = new AbortController();

    async function fetchVisibleGrade() {
      setGradesRefreshing(true);
      setLoading(!hasCachedVisibleGrade);
      setError('');
      try {
        const maybeResults = await apiSWR<GradeCache>('/api/grades/all', { signal: controller.signal });
        if (cancelled) return;
        if (maybeResults) {
          const allResults = normalizeGradeCache(maybeResults);
          const keys = gradeKeys(allResults);
          setExamOrder(keys);
          setGrades((current) => replaceGradeCache(current, allResults, cacheKey, gradesRef));
          if (keys.length > 0) {
            setExam(keys[0]);
          }
        }
      } catch (err) {
        if (cancelled) return;
        if (err instanceof DOMException && err.name === 'AbortError') return;
        if (isSessionExpired(err)) {
          clearGradeCache(currentSession.matricula);
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
      controller.abort();
    };
  }, [session]);

  const prefetchGrades = useCallback(() => {
    if (!session || gradesRefreshing || loading) return;
    const cacheKey = gradeCacheKey(session.matricula);
    const cached = readGradeCache(cacheKey);
    if (Object.keys(cached).length === 0) {
      void apiSWR<GradeCache>('/api/grades/all').catch(() => undefined);
      return;
    }
    void apiSWR<GradeCache>('/api/grades/all')
      .then((maybeResults) => {
        if (!maybeResults) return;
        const allResults = normalizeGradeCache(maybeResults);
        const keys = gradeKeys(allResults);
        setExamOrder(keys);
        setGrades((current) => replaceGradeCache(current, allResults, cacheKey, gradesRef));
        if (keys.length > 0) {
          setExam(keys[0]);
        }
      })
      .catch(() => undefined);
  }, [session, gradesRefreshing, loading]);

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

  function handleExamChange(nextExam: Exam) {
    setExam(nextExam);
    setActiveDetail(null);
  }

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
        clearGradeCache();
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
    clearGradeCache(session?.matricula);
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
      {!gradesRefreshing && <ExamSwitch exam={exam} exams={availableExams} labels={examLabels} carousel={useExamCarousel} setExam={handleExamChange} />}

      {error && <InlineError message={error} />}
      {loading && <div className="loading" role="status" aria-live="polite">Carregando notas...</div>}

      {hasRenderableTables ? (
        <section className="grade-list" id="grades" aria-live="polite">
          {activityTables.map((table) => (
            <GradeCard table={table} key={table.key} activeDetail={activeDetail} onToggleDetail={handleToggleDetail} onPrefetch={prefetchGrades} />
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
