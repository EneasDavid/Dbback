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
  const [activeDetail, setActiveDetail] = useState<{ tableKey: string; columnKey: string } | null>(null);
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
    Promise.allSettled([api<GradeResult>('/api/grades?exam=ab1'), api<GradeResult>('/api/grades?exam=ab2')])
      .then(([ab1Result, ab2Result]) => {
        const ab1 = ab1Result.status === 'fulfilled' ? normalizeGrade(ab1Result.value, 'AB1') : null;
        const ab2 = ab2Result.status === 'fulfilled' ? normalizeGrade(ab2Result.value, 'AB2') : null;
        setGrades({ ...(ab1 ? { ab1 } : {}), ...(ab2 ? { ab2 } : {}) });
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

      {loading && <div className="loading">Carregando notas...</div>}

      {!loading && visibleColumns.length > 0 && (
        <section className="grade-list">
          {visibleColumns.filter(shouldShowTable).map((table) =>
            table.kind === 'summary' ? (
              <SummaryTable table={table} key={table.key} />
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

function GradeCard({
  table,
  activeDetail,
  onToggleDetail,
}: {
  table: GradeTable;
  activeDetail: { tableKey: string; columnKey: string } | null;
  onToggleDetail: (tableKey: string, columnKey: string) => void;
}) {
  const activeKey = activeDetail?.tableKey === table.key ? activeDetail.columnKey : null;
  const comments = feedbackComments(table);
  return (
    <article className={`grade-table ${table.kind}`}>
      <header>
        <div>
          <h2>{table.label}</h2>
        </div>
      </header>
      {table.columns.filter((column) => shouldShowMainColumn(column, table)).map((column) => (
        <div key={`${table.key}-${column.key}`}>
          <GradeRow
            column={column}
            expanded={activeKey === column.key}
            onToggle={() => onToggleDetail(table.key, column.key)}
          />
          {activeKey === column.key && <GradeDetailPanel table={table} mainColumn={column} />}
        </div>
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
        <h2>{table.label}</h2>
      </header>
      <div className="summary-grid">
        {table.columns.filter(shouldShowColumn).map((column) => {
          const isAverage = isAverageColumn(column);
          return (
            <section
              className={`summary-score ${isAverage ? 'highlight' : ''} ${isAverage ? scoreTone(column) : ''}`}
              key={`${table.key}-${column.key}`}
            >
              <span>{normalized(column.label).includes('prova') ? 'Nota da prova' : isAverage ? 'Média da AB' : column.label}</span>
              <strong>{displayValue(column)}</strong>
              {column.comment && (
                <p>
                  <MessageSquareText size={15} />
                  {column.comment}
                </p>
              )}
            </section>
          );
        })}
      </div>
    </article>
  );
}

function GradeRow({
  column,
  expanded,
  onToggle,
}: {
  column: Column;
  expanded: boolean;
  onToggle: () => void;
}) {
  const clickable = isGradeColumn(column);
  return (
    <section className={`grade-row ${expanded ? 'expanded' : ''} ${clickable ? 'clickable' : ''} ${scoreTone(column)}`}>
      <button
        type="button"
        className="grade-row-trigger"
        onClick={clickable ? onToggle : undefined}
        aria-expanded={expanded}
        disabled={!clickable}
      >
        <div>
          <span>{column.label}</span>
          <strong>{displayValue(column)}</strong>
        </div>
        {clickable && <ChevronRight size={18} className={expanded ? 'rotated' : ''} />}
      </button>
      {column.comment && (
        <p className="row-comment">
          <MessageSquareText size={15} />
          {column.comment}
        </p>
      )}
    </section>
  );
}

function GradeDetailPanel({ table, mainColumn }: { table: GradeTable; mainColumn: Column }) {
  const compositionItems = table.columns.filter(
    (item) => shouldShowColumn(item) && item.key !== mainColumn.key && !isAverageColumn(item),
  );

  const parsedItems = compositionItems.map((item) => {
    const obtained = parseScore(item.value);
    const maxFromLabel = parseMaxFromLabel(item.label);
    const maxFromValue = parseMaxFromValue(item.value);
    const max = maxFromLabel ?? maxFromValue ?? 1;
    const ratio = obtained !== null && max > 0 ? Math.min((obtained / max) * 100, 100) : 0;

    return {
      ...item,
      obtained,
      max,
      ratio,
    };
  });

  const comments = [mainColumn.comment, ...compositionItems.map((item) => item.comment)].filter(Boolean) as string[];

  return (
    <section className="detail-panel">
      <div className="detail-header">
        <div>
          <span>Composição</span>
          <strong>{mainColumn.label}</strong>
        </div>
        <div className="detail-score">
          {displayValue(mainColumn)}
          <small>nota total</small>
        </div>
      </div>
      <div className="detail-items">
        {parsedItems.map((item) => (
          <article className="detail-item" key={item.key}>
            <div className="detail-item-row">
              <div>
                <strong>{item.label}</strong>
                <span>{item.value || '-'}</span>
              </div>
              {item.max ? (
                <span className="badge">{item.obtained !== null ? `${formatScore(item.obtained)} / ${formatScore(item.max)}` : `Max ${formatScore(item.max)}`}</span>
              ) : null}
            </div>
            <div className="detail-progress-bar" aria-hidden="true">
              <div className="detail-progress-fill" style={{ width: `${item.ratio}%` }} />
            </div>
          </article>
        ))}
      </div>
      {comments.length > 0 && (
        <div className="comment-bubble">
          <div className="comment-avatar">P</div>
          <div>
            <p>{comments[0]}</p>
            <span>Professor</span>
          </div>
        </div>
      )}
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
  for (const table of grade.tables ?? []) {
    for (const column of table.columns ?? []) {
      if (predicate(table, column)) {
        const score = parseScore(column.value);
        return score !== null && score > 0 ? score : null;
      }
    }
  }
  return null;
}

function normalizeGrade(grade: GradeResult, fallbackExam: string): GradeResult {
  return {
    ...grade,
    exam: grade.exam || fallbackExam,
    tables: Array.isArray(grade.tables)
      ? grade.tables.map((table) => ({
          ...table,
          columns: Array.isArray(table.columns) ? table.columns : [],
        }))
      : [],
  };
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
  return (
    label === 'nota' ||
    label.includes('prova') ||
    label.includes('nota ab') ||
    label === 'total' ||
    label.startsWith('semana') ||
    ['sgbd', 'dataset', 'crud', 'apresentacao', 'projeto'].includes(label)
  );
}

function isAverageColumn(column: Column) {
  const label = normalized(column.label);
  return label.includes('média') || label.includes('media') || label.includes('média da ab') || label.includes('media da ab');
}

function parseMaxFromLabel(label: string) {
  const match = label.match(/\[(\d+[.,]?\d*)\]/) || label.match(/\((\d+[.,]?\d*)\)/);
  return match ? parseScore(match[1]) : null;
}

function parseMaxFromValue(value: string) {
  if (!value.includes('/')) {
    return null;
  }
  const [, max] = value.split('/').map((part) => part.trim());
  return max ? parseScore(max) : null;
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
