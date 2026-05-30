import { StrictMode, useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { AlertCircle, BookOpenCheck, ChevronRight, LogOut, MessageSquareText, Search } from 'lucide-react';
import './styles.css';

type Column = {
  key: string;
  label: string;
  value: string;
  comment?: string;
};

type GradeResult = {
  exam: string;
  sheetName: string;
  matricula: string;
  columns: Column[];
};

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init?.headers || {}) },
    ...init,
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || 'Nao foi possivel concluir a operacao.');
  }
  return payload as T;
}

function App() {
  const [matricula, setMatricula] = useState('');
  const [session, setSession] = useState<string | null>(null);
  const [exam, setExam] = useState<'ab1' | 'ab2'>('ab1');
  const [grade, setGrade] = useState<GradeResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    api<{ matricula: string }>('/api/me')
      .then((me) => setSession(me.matricula))
      .catch(() => setSession(null));
  }, []);

  useEffect(() => {
    if (!session) return;
    setLoading(true);
    setError('');
    api<GradeResult>(`/api/grades?exam=${exam}`)
      .then(setGrade)
      .catch((err) => {
        setGrade(null);
        setError(err.message);
      })
      .finally(() => setLoading(false));
  }, [session, exam]);

  const visibleColumns = useMemo(() => {
    return grade?.columns.filter((column) => column.label.trim() !== '') ?? [];
  }, [grade]);

  async function handleLogin(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      const result = await api<{ matricula: string }>('/api/login', {
        method: 'POST',
        body: JSON.stringify({ matricula }),
      });
      setSession(result.matricula);
      setMatricula('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Erro ao entrar.');
    } finally {
      setLoading(false);
    }
  }

  async function handleLogout() {
    await api('/api/logout', { method: 'POST' }).catch(() => null);
    setSession(null);
    setGrade(null);
    setError('');
  }

  if (!session) {
    return (
      <main className="shell login-shell">
        <section className="login-panel">
          <div className="brand-mark">
            <BookOpenCheck size={34} strokeWidth={2.2} />
          </div>
          <h1>Feedback de Notas</h1>
          <p>Acesse com a sua matricula para consultar AB1 ou AB2.</p>
          <form onSubmit={handleLogin}>
            <label htmlFor="matricula">Matricula</label>
            <div className="field">
              <Search size={18} />
              <input
                id="matricula"
                inputMode="numeric"
                autoComplete="username"
                placeholder="Digite sua matricula"
                value={matricula}
                onChange={(event) => setMatricula(event.target.value)}
              />
            </div>
            {error && <InlineError message={error} />}
            <button className="primary-button" type="submit" disabled={loading}>
              {loading ? 'Entrando...' : 'Entrar'}
              <ChevronRight size={18} />
            </button>
          </form>
        </section>
      </main>
    );
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <span>Matricula</span>
          <strong>{session}</strong>
        </div>
        <button className="icon-button" type="button" onClick={handleLogout} aria-label="Sair">
          <LogOut size={18} />
        </button>
      </header>

      <section className="exam-switch" aria-label="Selecionar avaliacao">
        <button className={exam === 'ab1' ? 'active' : ''} type="button" onClick={() => setExam('ab1')}>
          AB1
        </button>
        <button className={exam === 'ab2' ? 'active' : ''} type="button" onClick={() => setExam('ab2')}>
          AB2
        </button>
      </section>

      {error && <InlineError message={error} />}

      <section className="result-head">
        <span>{grade?.sheetName || (exam === 'ab1' ? 'Notas AB1' : 'Projeto AB2')}</span>
        <h1>{exam.toUpperCase()}</h1>
      </section>

      {loading && <div className="loading">Carregando notas...</div>}

      {!loading && visibleColumns.length > 0 && (
        <section className="grade-list">
          {visibleColumns.map((column) => (
            <article className="grade-row" key={column.key}>
              <div>
                <span>{column.label}</span>
                <strong>{column.value || '-'}</strong>
              </div>
              {column.comment && (
                <p>
                  <MessageSquareText size={15} />
                  {column.comment}
                </p>
              )}
            </article>
          ))}
        </section>
      )}
    </main>
  );
}

function InlineError({ message }: { message: string }) {
  return (
    <div className="inline-error" role="alert">
      <AlertCircle size={17} />
      <span>{message}</span>
    </div>
  );
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
