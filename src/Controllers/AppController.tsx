import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { EmptyState, ExamSwitch, GradeCard, InlineError, LoginView, ReaderGradeDocument, SummaryTable, Topbar } from '../Views/components';
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
import { api, apiSWR, clearApiValidators } from './apiController';

const EMPTY_STATE_MS = 5_000;
const GRADES_REVALIDATE_MS = 30_000;
const GRADES_PATH = '/api/grades/all';
const LAST_MATRICULA_KEY = 'dbback-last-matricula';
const THEME_QUERY = '(prefers-color-scheme: dark)';

type Theme = 'light' | 'dark';
type Exam = string;
type GradeActivationMode = 'replace' | 'store';

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
  const gradesFetchedAtRef = useRef(0);
  const [activeDetail, setActiveDetail] = useState<{ tableKey: string; cardKey: string } | null>(null);
  const [loading, setLoading] = useState(false);
  const [gradesRefreshing, setGradesRefreshing] = useState(false);
  const [showEmptyState, setShowEmptyState] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    gradesRef.current = grades;
  }, [grades]);

  const activateGradeResults = useCallback((incoming: GradeCache, cacheKey: string, mode: GradeActivationMode = 'replace') => {
    const allResults = normalizeGradeCache(incoming);
    const keys = gradeKeys(allResults);

    setExamOrder(keys);
    setGrades((current) =>
      mode === 'replace'
        ? replaceGradeCache(current, allResults, cacheKey, gradesRef)
        : storeGradeCache(current, allResults, cacheKey, gradesRef),
    );
    setExam((currentExam) => (keys.length > 0 && !allResults[currentExam] ? keys[0] : currentExam));

    return keys.length > 0;
  }, []);

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
        clearGradeCache();
        clearApiValidators([GRADES_PATH]);
        setSession(null);
      })
      .catch(() => {
        clearGradeCache();
        clearApiValidators([GRADES_PATH]);
        setSession(null);
      });
  }, []);

  useEffect(() => {
    if (!session) return;
    const cacheKey = gradeCacheKey(session.matricula);
    const cached = readGradeCache(cacheKey);
    if (Object.keys(cached).length > 0) {
      activateGradeResults(cached, cacheKey, 'store');
    }
  }, [activateGradeResults, session]);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    const currentSession = session;
    const cacheKey = gradeCacheKey(currentSession.matricula);
    const cached = readGradeCache(cacheKey);
    const cachedGrades = { ...cached, ...gradesRef.current };
    const hasCachedVisibleGrade = Object.values(cachedGrades).some((grade) => hasRenderableGrade(grade));

    async function fetchVisibleGrade() {
      setGradesRefreshing(true);
      setLoading(!hasCachedVisibleGrade);
      setError('');
      try {
        let maybeResults = await apiSWR<GradeCache>(GRADES_PATH);
        if (!maybeResults && !hasCachedVisibleGrade) {
          clearApiValidators([GRADES_PATH]);
          maybeResults = await apiSWR<GradeCache>(GRADES_PATH, { cache: 'reload' });
        }
        if (cancelled) return;
        gradesFetchedAtRef.current = Date.now();
        if (maybeResults) {
          activateGradeResults(maybeResults, cacheKey);
        } else if (hasCachedVisibleGrade) {
          activateGradeResults(cachedGrades, cacheKey, 'store');
        } else {
          setError('Nao foi possivel revalidar as notas. Tente entrar novamente.');
        }
      } catch (err) {
        if (cancelled) return;
        if (err instanceof DOMException && err.name === 'AbortError') return;
        if (isSessionExpired(err)) {
          clearGradeCache(currentSession.matricula);
          clearApiValidators([GRADES_PATH]);
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
  }, [activateGradeResults, session]);

  const prefetchGrades = useCallback(() => {
    if (!session || gradesRefreshing || loading) return;
    const cacheKey = gradeCacheKey(session.matricula);
    if (Date.now() - gradesFetchedAtRef.current < GRADES_REVALIDATE_MS) {
      return;
    }
    gradesFetchedAtRef.current = Date.now();
    void apiSWR<GradeCache>(GRADES_PATH)
      .then((maybeResults) => {
        if (!maybeResults) return;
        activateGradeResults(maybeResults, cacheKey);
      })
      .catch(() => {
        gradesFetchedAtRef.current = 0;
      });
  }, [activateGradeResults, session, gradesRefreshing, loading]);

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
  const currentExamLabel = examLabels[exam] || exam.toUpperCase();

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
        clearApiValidators([GRADES_PATH]);
      }
      window.localStorage.setItem(LAST_MATRICULA_KEY, resolvedMatricula);
      gradesFetchedAtRef.current = 0;
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
    clearApiValidators([GRADES_PATH]);
    gradesFetchedAtRef.current = 0;
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
        <>
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
          <ReaderGradeDocument session={session} examLabel={currentExamLabel} tables={[...activityTables, ...summaryTables, ...mediaTables]} />
        </>
      ) : (
        !loading && showEmptyState && <EmptyState exam={exam} />
      )}
    </main>
  );
}
