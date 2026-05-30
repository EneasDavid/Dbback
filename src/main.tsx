import { StrictMode, useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { AlertCircle, BookOpenCheck, ChevronRight, LogOut, MessageSquareText, Moon, Search, Sun } from 'lucide-react';
import './styles.css';

type Column = {
  key: string;
  label: string;
  value: string;
  comment?: string;
};

type GradeResult = {
  exam: string;
  matricula: string;
  name: string;
  tables: GradeTable[];
};

type SessionUser = {
  matricula: string;
  name: string;
};

type GradeTable = {
  key: string;
  label: string;
  sheetName: string;
  kind: string;
  complete: boolean;
  columns: Column[];
};

type StudentStatus = {
  ab1: number;
  ab2: number;
  average: number;
  approved: boolean;
};

type GradeCache = Partial<Record<'ab1' | 'ab2', GradeResult>>;

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
  const [session, setSession] = useState<SessionUser | null>(null);
  const [exam, setExam] = useState<'ab1' | 'ab2'>('ab1');
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    return window.localStorage.getItem('theme') === 'dark' ? 'dark' : 'light';
  });
  const [grades, setGrades] = useState<GradeCache>({});
  const [studentStatus, setStudentStatus] = useState<StudentStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', theme === 'dark' ? '#020617' : '#0f172a');
    window.localStorage.setItem('theme', theme);
  }, [theme]);

  useEffect(() => {
    api<SessionUser>('/api/me')
      .then(setSession)
      .catch(() => setSession(null));
  }, []);

  useEffect(() => {
    if (!session) return;
    setLoading(true);
    setError('');
    Promise.all([api<GradeResult>('/api/grades?exam=ab1'), api<GradeResult>('/api/grades?exam=ab2')])
      .then(([ab1, ab2]) => {
        setGrades({ ab1, ab2 });
        const ab1Total = findScore(ab1, (table, column) => table.complete && table.kind === 'summary' && normalized(column.label).includes('nota') && normalized(column.label).includes('ab'));
        const ab2Total = findScore(ab2, (table, column) => table.complete && table.key === 'projeto' && normalized(column.label) === 'total');
        if (ab1Total === null || ab2Total === null) {
          setStudentStatus(null);
          return;
        }
        const average = (ab1Total + ab2Total) / 2;
        setStudentStatus({ ab1: ab1Total, ab2: ab2Total, average, approved: average >= 7 });
      })
      .catch((err) => {
        setGrades({});
        setStudentStatus(null);
        setError(err instanceof Error ? err.message : 'Erro ao carregar as notas.');
      })
      .finally(() => setLoading(false));
  }, [session]);

  const visibleColumns = useMemo(() => {
    return grades[exam]?.tables ?? [];
  }, [grades, exam]);

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
    setSession(null);
    setGrades({});
    setStudentStatus(null);
    setError('');
  }

  if (!session) {
    return (
      <main className="shell login-shell">
        <ThemeButton theme={theme} setTheme={setTheme} />
        <section className="login-panel">
          <div className="brand-mark">
            <BookOpenCheck size={34} strokeWidth={2.2} />
          </div>
          <h1>dbBack</h1>
          <p>Use sua matricula da UFAL para acessar suas notas e feedbacks das atividades.</p>
          <form onSubmit={handleLogin}>
            <label htmlFor="matricula">Matricula UFAL</label>
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
          <span>{session.matricula}</span>
          <strong>{session.name || 'Aluno'}</strong>
        </div>
        <button className="icon-button" type="button" onClick={handleLogout} aria-label="Sair">
          <LogOut size={18} />
        </button>
        <ThemeButton theme={theme} setTheme={setTheme} compact />
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

      {studentStatus && <StatusBanner status={studentStatus} />}

      {loading && <div className="loading">Carregando notas...</div>}

      {!loading && visibleColumns.length > 0 && (
        <section className="grade-list">
          {visibleColumns.filter(shouldShowTable).map((table) => (table.kind === 'summary' ? <SummaryTable table={table} key={table.key} /> : <GradeCard table={table} key={table.key} />))}
        </section>
      )}
    </main>
  );
}

function GradeCard({ table }: { table: GradeTable }) {
  const comments = feedbackComments(table);
  return (
    <article className={`grade-table ${table.kind}`}>
      <header>
        <h2>{table.label}</h2>
      </header>
      {table.columns.filter(shouldShowColumn).map((column) => (
        <GradeRow column={column} key={`${table.key}-${column.key}`} />
      ))}
      {comments.length > 0 && (
        <section className="feedback-list">
          <span>Feedback</span>
          {comments.map((comment, index) => (
            <p key={`${table.key}-comment-${index}`}>
              <MessageSquareText size={15} />
              {comment}
            </p>
          ))}
        </section>
      )}
    </article>
  );
}

function SummaryTable({ table }: { table: GradeTable }) {
  return (
    <article className="grade-table summary">
      <header>
        <h2>Média AB1</h2>
      </header>
      <div className="summary-grid">
        {table.columns.filter(shouldShowColumn).map((column) => (
          <section className={`summary-score ${scoreTone(column)}`} key={`${table.key}-${column.key}`}>
            <span>{normalized(column.label).includes('prova') ? 'Nota da prova' : 'Média da AB'}</span>
            <strong>{displayValue(column)}</strong>
            {column.comment && (
              <p>
                <MessageSquareText size={15} />
                {column.comment}
              </p>
            )}
          </section>
        ))}
      </div>
    </article>
  );
}

function GradeRow({ column }: { column: Column }) {
  return (
    <section className={`grade-row ${scoreTone(column)}`}>
      <div>
        <span>{column.label}</span>
        <strong>{displayValue(column)}</strong>
      </div>
      {column.comment && (
        <p>
          <MessageSquareText size={15} />
          {column.comment}
        </p>
      )}
    </section>
  );
}

function StatusBanner({ status }: { status: StudentStatus }) {
  return (
    <section className={`status-banner ${status.approved ? 'approved' : 'pending'}`}>
      <span>{status.approved ? 'Aprovado' : 'Media parcial'}</span>
      <strong>{formatScore(status.average)}</strong>
      <p>AB1 {formatScore(status.ab1)} + AB2 {formatScore(status.ab2)}</p>
    </section>
  );
}

function ThemeButton({
  theme,
  setTheme,
  compact = false,
}: {
  theme: 'light' | 'dark';
  setTheme: (theme: 'light' | 'dark') => void;
  compact?: boolean;
}) {
  const next = theme === 'dark' ? 'light' : 'dark';
  return (
    <button className={compact ? 'icon-button theme-compact' : 'theme-button'} type="button" onClick={() => setTheme(next)} aria-label="Alternar modo noturno">
      {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
      {!compact && <span>{theme === 'dark' ? 'Modo claro' : 'Modo noturno'}</span>}
    </button>
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

function shouldShowColumn(column: Column) {
  const label = normalized(column.label);
  return label !== '' && label !== 'grupo' && !label.includes('matricula') && !label.includes('nome do aluno') && label !== 'nome' && label !== 'aluno';
}

function shouldShowTable(table: GradeTable) {
  return table.kind === 'summary' || table.columns.some(shouldShowColumn) || feedbackComments(table).length > 0;
}

function feedbackComments(table: GradeTable) {
  const seen = new Set<string>();
  const comments: string[] = [];
  for (const column of table.columns) {
    const comment = column.comment?.trim();
    if (comment && !seen.has(comment)) {
      seen.add(comment);
      comments.push(comment);
    }
  }
  return comments;
}

function scoreTone(column: Column) {
  const label = normalized(column.label);
  if (!label.includes('nota ab')) return '';
  const value = parseScore(column.value);
  if (value === null) return '';
  if (value < 5) return 'score-danger';
  if (value < 7) return 'score-warning';
  return 'score-success';
}

function findScore(grade: GradeResult, predicate: (table: GradeTable, column: Column) => boolean) {
  for (const table of grade.tables) {
    for (const column of table.columns) {
      if (predicate(table, column)) {
        const score = parseScore(column.value);
        return score !== null && score > 0 ? score : null;
      }
    }
  }
  return null;
}

function parseScore(value: string) {
  const parsed = Number(value.replace(',', '.').replace(/[^\d.-]/g, ''));
  return Number.isFinite(parsed) ? parsed : null;
}

function displayValue(column: Column) {
  if (isGradeColumn(column)) {
    const score = parseScore(column.value);
    if (score === null) {
      return 'Não corrigida ainda';
    }
  }
  return column.value || '-';
}

function isGradeColumn(column: Column) {
  const label = normalized(column.label);
  return label === 'nota' || label.includes('prova') || label.includes('nota ab') || label === 'total' || label.startsWith('semana') || ['sgbd', 'dataset', 'crud', 'apresentacao'].includes(label);
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

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
