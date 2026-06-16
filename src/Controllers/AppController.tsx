import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { EmptyState, ExamSwitch, GradeCard, InlineError, LoginView, ReaderGradeDocument, ScrollTopButton, SummaryTable, Topbar } from '../Views/components';
import {
  cardsFor,
  clearGradeCache,
  gradeCacheKey,
  gradeKeys,
  gradeLabels,
  hasRenderableGrade,
  isSessionExpired,
  isSummaryTable,
  matriculasDiffer,
  normalizeGradeCache,
  readGradeCache,
  replaceGradeCache,
  storeGradeCache,
} from '../Models/gradeModel';
import type { GradeCache, GradeResult, SessionUser } from '../Models/types';
import { api, apiSWR, clearApiValidators } from './apiController';

const EMPTY_STATE_MS = 5_000;
const GRADES_REVALIDATE_MS = 30_000;
const DEFAULT_EXAM = 'ab1';
const ALL_GRADES_PATH = '/api/grades/all';
const LAST_MATRICULA_KEY = 'dbback-last-matricula';
const THEME_QUERY = '(prefers-color-scheme: dark)';
const MULTI_DETAIL_QUERY = '(min-width: 768px), (horizontal-viewport-segments: 2)';
const TURNSTILE_SITE_KEY = import.meta.env.VITE_TURNSTILE_SITE_KEY?.trim() ?? '';
const TURNSTILE_CONFIG_MESSAGE = 'VITE_TURNSTILE_SITE_KEY nao configurada. Configure o Cloudflare Turnstile para liberar o login.';

type Theme = 'light' | 'dark';
type Exam = string;
type GradeActivationMode = 'replace' | 'store';
type ActiveDetails = Record<string, string>;

function systemTheme(): Theme {
  return window.matchMedia?.(THEME_QUERY).matches ? 'dark' : 'light';
}

function initialTheme(): Theme {
  return systemTheme();
}

function initialMultiDetailMode() {
  return window.matchMedia?.(MULTI_DETAIL_QUERY).matches ?? false;
}

function gradePath(exam: Exam) {
  return `/api/grades?exam=${encodeURIComponent(normalizeExamKey(exam))}`;
}

function normalizeExamKey(exam: Exam) {
  return exam.trim().toLowerCase() || DEFAULT_EXAM;
}

function clearGradeValidators() {
  clearApiValidators();
}

export default function AppController() {
  const [matricula, setMatricula] = useState(() => window.localStorage.getItem(LAST_MATRICULA_KEY) || '');
  const [session, setSession] = useState<SessionUser | null>(null);
  const [exam, setExam] = useState<Exam>(DEFAULT_EXAM);
  const [examOrder, setExamOrder] = useState<Exam[]>([]);
  const [theme, setTheme] = useState<Theme>(() => initialTheme());
  const [grades, setGrades] = useState<GradeCache>({});
  const gradesRef = useRef<GradeCache>({});
  const gradesFetchedAtRef = useRef(0);
  const allGradesLoadingRef = useRef(false);
  const [activeDetails, setActiveDetails] = useState<ActiveDetails>({});
  const [multiDetailMode, setMultiDetailMode] = useState(() => initialMultiDetailMode());
  const [turnstileToken, setTurnstileToken] = useState('');
  const [turnstileResetKey, setTurnstileResetKey] = useState(0);
  const [loading, setLoading] = useState(false);
  const [gradesRefreshing, setGradesRefreshing] = useState(false);
  const [showEmptyState, setShowEmptyState] = useState(false);
  const [error, setError] = useState('');
  const gradeListRef = useRef<HTMLElement>(null);

  useEffect(() => {
    gradesRef.current = grades;
  }, [grades]);

  useEffect(() => {
    const media = window.matchMedia?.(MULTI_DETAIL_QUERY);
    if (!media) return;

    const syncMultiDetailMode = () => {
      setMultiDetailMode(media.matches);
      if (!media.matches) {
        setActiveDetails((current) => {
          const openDetails = Object.entries(current);
          const lastOpenDetail = openDetails[openDetails.length - 1];
          return lastOpenDetail ? { [lastOpenDetail[0]]: lastOpenDetail[1] } : {};
        });
      }
    };

    syncMultiDetailMode();

    if (media.addEventListener) {
      media.addEventListener('change', syncMultiDetailMode);
      return () => media.removeEventListener('change', syncMultiDetailMode);
    }
    media.addListener(syncMultiDetailMode);
    return () => media.removeListener(syncMultiDetailMode);
  }, []);

  const activateGradeResults = useCallback((incoming: GradeCache, cacheKey: string, mode: GradeActivationMode = 'replace') => {
    const allResults = normalizeGradeCache(incoming);
    const visibleResults = mode === 'replace' ? allResults : normalizeGradeCache({ ...gradesRef.current, ...allResults });
    const keys = gradeKeys(visibleResults);

    setExamOrder(keys);
    setGrades((current) =>
      mode === 'replace'
        ? replaceGradeCache(current, allResults, cacheKey, gradesRef)
        : storeGradeCache(current, allResults, cacheKey, gradesRef),
    );
    setExam((currentExam) => {
      if (keys.length === 0) return currentExam;
      if (!keys.includes(currentExam)) return keys[0];
      if (!hasRenderableGrade(visibleResults[currentExam])) {
        return keys.find((key) => hasRenderableGrade(visibleResults[key])) || currentExam;
      }
      return currentExam;
    });

    return keys.length > 0;
  }, []);

  const prefetchAllGrades = useCallback((cacheKey: string, force = false) => {
    if (allGradesLoadingRef.current) return;
    if (!force && Date.now() - gradesFetchedAtRef.current < GRADES_REVALIDATE_MS) {
      return;
    }

    gradesFetchedAtRef.current = Date.now();
    allGradesLoadingRef.current = true;
    setGradesRefreshing(true);

    void apiSWR<GradeCache>(ALL_GRADES_PATH, force ? { cache: 'reload' } : undefined)
      .then((maybeResults) => {
        if (!maybeResults) return;
        activateGradeResults(maybeResults, cacheKey, 'replace');
      })
      .catch((err) => {
        gradesFetchedAtRef.current = 0;
        if (isSessionExpired(err)) {
          clearGradeCache();
          clearGradeValidators();
          setGrades({});
          setSession(null);
        }
      })
      .finally(() => {
        allGradesLoadingRef.current = false;
        setGradesRefreshing(false);
      });
  }, [activateGradeResults]);

  useEffect(() => {
    if (!session) return;
    const cacheKey = gradeCacheKey(session.matricula);
    const timer = window.setInterval(() => prefetchAllGrades(cacheKey, true), GRADES_REVALIDATE_MS);
    return () => window.clearInterval(timer);
  }, [prefetchAllGrades, session]);

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
        clearGradeValidators();
        setSession(null);
      })
      .catch(() => {
        clearGradeCache();
        clearGradeValidators();
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
    let backgroundTimer = 0;
    const currentSession = session;
    const currentExam = normalizeExamKey(exam);
    const currentGradePath = gradePath(currentExam);
    const cacheKey = gradeCacheKey(currentSession.matricula);
    const cached = readGradeCache(cacheKey);
    const cachedGrades = { ...cached, ...gradesRef.current };
    const hasCachedVisibleGrade = hasRenderableGrade(cachedGrades[currentExam]);

    async function fetchVisibleGrade() {
      setGradesRefreshing(true);
      setLoading(!hasCachedVisibleGrade);
      setError('');
      try {
        let maybeResult = await apiSWR<GradeResult>(currentGradePath);
        if (!maybeResult && !hasCachedVisibleGrade) {
          clearApiValidators([currentGradePath]);
          maybeResult = await apiSWR<GradeResult>(currentGradePath, { cache: 'reload' });
        }
        if (cancelled) return;
        if (maybeResult) {
          activateGradeResults({ [currentExam]: maybeResult }, cacheKey, 'store');
        } else if (hasCachedVisibleGrade) {
          activateGradeResults({ [currentExam]: cachedGrades[currentExam] }, cacheKey, 'store');
        } else {
          setError('Nao foi possivel revalidar as notas. Tente entrar novamente.');
        }
      } catch (err) {
        if (cancelled) return;
        if (err instanceof DOMException && err.name === 'AbortError') return;
        if (isSessionExpired(err)) {
          clearGradeCache(currentSession.matricula);
          clearGradeValidators();
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
          backgroundTimer = window.setTimeout(() => {
            if (!cancelled) {
              prefetchAllGrades(cacheKey);
            }
          }, 80);
        }
      }
    }

    void fetchVisibleGrade();

    return () => {
      cancelled = true;
      window.clearTimeout(backgroundTimer);
    };
  }, [activateGradeResults, exam, prefetchAllGrades, session]);

  const prefetchGrades = useCallback(() => {
    if (!session || gradesRefreshing || loading) return;
    const cacheKey = gradeCacheKey(session.matricula);
    prefetchAllGrades(cacheKey);
  }, [prefetchAllGrades, session, gradesRefreshing, loading]);

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

  const renderableTables = useMemo(() => visibleTables.filter((table) => cardsFor(table).length > 0), [visibleTables]);
  const hasRenderableTables = renderableTables.length > 0;
  const currentExamLabel = examLabels[exam] || exam.toUpperCase();

  useLayoutEffect(() => {
    const container = gradeListRef.current;
    if (!container) return;

    let frame = 0;

    const resetItems = () => {
      masonryElements(container).forEach((item) => {
        item.style.gridRowEnd = '';
      });
    };

    const updateRows = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(() => {
        const styles = window.getComputedStyle(container);
        const columns = styles.gridTemplateColumns.split(' ').filter(Boolean).length;
        if (columns <= 1) {
          resetItems();
          return;
        }

        const rowHeight = parseFloat(styles.getPropertyValue('--masonry-row-height')) || 8;
        const rowGap = parseFloat(styles.rowGap) || 0;
        const rowSize = rowHeight + rowGap;

        masonryElements(container).forEach((item) => {
          item.style.gridRowEnd = '';
          const height = item.getBoundingClientRect().height;
          const rows = Math.max(1, Math.ceil((height + rowGap) / rowSize));
          item.style.gridRowEnd = `span ${rows}`;
        });
      });
    };

    updateRows();
    window.addEventListener('resize', updateRows);

    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(updateRows);
    observer?.observe(container);
    masonryElements(container).forEach((item) => observer?.observe(item));

    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener('resize', updateRows);
      observer?.disconnect();
      resetItems();
    };
  }, [activeDetails, exam, renderableTables]);

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
    setActiveDetails((current) => {
      if (current[tableKey] === cardKey) {
        const next = { ...current };
        delete next[tableKey];
        return next;
      }
      if (!multiDetailMode) {
        return { [tableKey]: cardKey };
      }
      return { ...current, [tableKey]: cardKey };
    });
  };

  function handleExamChange(nextExam: Exam) {
    setExam(nextExam);
    setActiveDetails({});
  }

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalizedMatricula = matricula.trim();
    if (!TURNSTILE_SITE_KEY) {
      setError(TURNSTILE_CONFIG_MESSAGE);
      return;
    }
    if (TURNSTILE_SITE_KEY && !turnstileToken) {
      setError('Confirme que voce nao e um robo.');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const result = await api<SessionUser>('/api/login', {
        method: 'POST',
        body: JSON.stringify({ matricula: normalizedMatricula, turnstileToken }),
      });
      const previousMatricula = window.localStorage.getItem(LAST_MATRICULA_KEY) || '';
      const resolvedMatricula = result.matricula || normalizedMatricula;
      if (matriculasDiffer(previousMatricula, resolvedMatricula)) {
        clearGradeCache();
        clearGradeValidators();
      }
      window.localStorage.setItem(LAST_MATRICULA_KEY, resolvedMatricula);
      gradesFetchedAtRef.current = 0;
      allGradesLoadingRef.current = false;
      gradesRef.current = {};
      setGrades({});
      setExamOrder([]);
      setActiveDetails({});
      setTurnstileToken('');
      setTurnstileResetKey((current) => current + 1);
      setExam(DEFAULT_EXAM);
      setSession(result);
      setMatricula('');
    } catch (err) {
      setTurnstileToken('');
      setTurnstileResetKey((current) => current + 1);
      setError(err instanceof Error ? err.message : 'Erro ao entrar.');
    } finally {
      setLoading(false);
    }
  }

  async function handleLogout() {
    await api('/api/logout', { method: 'POST' }).catch(() => null);
    clearGradeCache(session?.matricula);
    clearGradeValidators();
    gradesFetchedAtRef.current = 0;
    allGradesLoadingRef.current = false;
    setSession(null);
    setGrades({});
    setExamOrder([]);
    setActiveDetails({});
    setTurnstileToken('');
    setTurnstileResetKey((current) => current + 1);
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
        error={error || (!TURNSTILE_SITE_KEY ? TURNSTILE_CONFIG_MESSAGE : '')}
        theme={theme}
        setTheme={handleThemeChange}
        onSubmit={handleLogin}
        turnstileSiteKey={TURNSTILE_SITE_KEY}
        turnstileVerified={Boolean(TURNSTILE_SITE_KEY && turnstileToken)}
        turnstileResetKey={turnstileResetKey}
        onTurnstileToken={setTurnstileToken}
        onTurnstileExpire={() => setTurnstileToken('')}
        onTurnstileError={() => {
          setTurnstileToken('');
          setError('Nao foi possivel confirmar a verificacao anti-robo. Tente novamente.');
        }}
      />
    );
  }

  return (
    <>
      <main className="shell">
        <Topbar session={session} theme={theme} setTheme={handleThemeChange} onLogout={handleLogout} />
        <ExamSwitch exam={exam} exams={availableExams} labels={examLabels} carousel={useExamCarousel} setExam={handleExamChange} />

        {error && <InlineError message={error} />}
        {loading && <div className="loading" role="status" aria-live="polite">Carregando notas...</div>}

        {hasRenderableTables ? (
          <>
            <section className="grade-list" id="grades" aria-live="polite" ref={gradeListRef}>
              {renderableTables.map((table) =>
                isSummaryTable(table.kind) ? (
                  <SummaryTable table={table} key={table.key} />
                ) : (
                  <GradeCard
                    table={table}
                    key={table.key}
                    activeDetail={activeDetails[table.key] ? { tableKey: table.key, cardKey: activeDetails[table.key] } : null}
                    onToggleDetail={handleToggleDetail}
                    onPrefetch={prefetchGrades}
                  />
                )
              )}
            </section>
            <ReaderGradeDocument session={session} examLabel={currentExamLabel} tables={renderableTables} />
          </>
        ) : (
          !loading && showEmptyState && <EmptyState exam={exam} />
        )}
      </main>
      <ScrollTopButton />
    </>
  );
}

function masonryElements(container: HTMLElement) {
  return Array.from(container.children).filter((child): child is HTMLElement => child instanceof HTMLElement);
}
