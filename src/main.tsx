import { StrictMode, useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { api } from './api';
import { AB2Summary, ExamSwitch, GradeCard, InlineError, LoginView, SummaryTable, Topbar } from './components';
import { computeStudentStatus, normalizeGrade, shouldShowTable } from './gradeUtils';
import type { GradeCache, GradeResult, SessionUser, StudentStatus } from './types';
import './styles.scss';

function App() {
  const [matricula, setMatricula] = useState('');
  const [session, setSession] = useState<SessionUser | null>(null);
  const [exam, setExam] = useState<'ab1' | 'ab2'>('ab1');
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    return window.localStorage.getItem('theme') === 'dark' ? 'dark' : 'light';
  });
  const [grades, setGrades] = useState<GradeCache>({});
  const [studentStatus, setStudentStatus] = useState<StudentStatus | null>(null);
  const [activeDetail, setActiveDetail] = useState<{ tableKey: string; columnKey: string } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!error) return;
    const timeout = window.setTimeout(() => setError(''), 10_000);
    return () => window.clearTimeout(timeout);
  }, [error]);

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
    const cacheKey = `dbback-grades:${session.matricula}`;
    const cached = window.sessionStorage.getItem(cacheKey);
    if (cached) {
      try {
        setGrades(JSON.parse(cached) as GradeCache);
      } catch {
        window.sessionStorage.removeItem(cacheKey);
      }
    }
    setLoading(true);
    setError('');
    const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined;
    const refresh = navigation?.type === 'reload' ? '&refresh=1' : '';
    Promise.allSettled([api<GradeResult>(`/api/grades?exam=ab1${refresh}`), api<GradeResult>(`/api/grades?exam=ab2${refresh}`)])
      .then(([ab1Result, ab2Result]) => {
        const ab1 = ab1Result.status === 'fulfilled' ? normalizeGrade(ab1Result.value, 'AB1') : null;
        const ab2 = ab2Result.status === 'fulfilled' ? normalizeGrade(ab2Result.value, 'AB2') : null;
        const expired = [ab1Result, ab2Result].some((result) => result.status === 'rejected' && isSessionExpired(result.reason));
        if (expired) {
          clearClientSession(session.matricula);
          setGrades({});
          setStudentStatus(null);
          setError('');
          setSession(null);
          return;
        }
        const nextGrades = { ...(ab1 ? { ab1 } : {}), ...(ab2 ? { ab2 } : {}) };
        window.sessionStorage.setItem(cacheKey, JSON.stringify(nextGrades));
        setGrades((current) => (JSON.stringify(current) === JSON.stringify(nextGrades) ? current : nextGrades));
        if (!ab1 && !ab2) {
          const reason = ab1Result.status === 'rejected' ? ab1Result.reason : ab2Result.status === 'rejected' ? ab2Result.reason : null;
          setError(reason instanceof Error ? reason.message : 'Nao foi possivel carregar as notas.');
          return;
        }
        if (!ab1 || !ab2) {
          setStudentStatus(null);
          setError('Algumas notas ainda nao estao disponiveis.');
          return;
        }
        setStudentStatus(computeStudentStatus(ab1, ab2));
      })
      .catch((err) => {
        if (isSessionExpired(err)) {
          clearClientSession(session.matricula);
          setGrades({});
          setStudentStatus(null);
          setError('');
          setSession(null);
          return;
        }
        setGrades({});
        setStudentStatus(null);
        setError(err instanceof Error ? err.message : 'Erro ao carregar as notas.');
      })
      .finally(() => setLoading(false));
  }, [session]);

  const visibleColumns = useMemo(() => {
    return grades[exam]?.tables ?? [];
  }, [grades, exam]);

  const handleToggleDetail = (tableKey: string, columnKey: string) => {
    setActiveDetail((current) =>
      current?.tableKey === tableKey && current.columnKey === columnKey ? null : { tableKey, columnKey }
    );
  };

  async function handleLogin(event: React.FormEvent<HTMLFormElement>) {
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
    setStudentStatus(null);
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
      {loading && <div className="loading">Carregando notas...</div>}

      {!loading && visibleColumns.length > 0 && (
        <section className="grade-list">
          {exam === 'ab2' && studentStatus && <AB2Summary status={studentStatus} />}
          {visibleColumns.filter(shouldShowTable).map((table) =>
            table.kind === 'summary' ? (
              <SummaryTable table={table} key={table.key} exam={exam} status={studentStatus} />
            ) : (
              <GradeCard
                table={table}
                key={table.key}
                activeDetail={activeDetail}
                onToggleDetail={handleToggleDetail}
              />
            )
          )}
        </section>
      )}
    </main>
  );
}

function clearClientSession(matricula?: string) {
  if (matricula) {
    window.sessionStorage.removeItem(`dbback-grades:${matricula}`);
  }
  if (!matricula) {
    for (const key of Object.keys(window.sessionStorage)) {
      if (key.startsWith('dbback-grades:')) window.sessionStorage.removeItem(key);
    }
  }
}

function isSessionExpired(error: unknown) {
  return error instanceof Error && error.message.toLowerCase().includes('sessao expirada');
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
