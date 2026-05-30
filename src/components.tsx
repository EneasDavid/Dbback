import type { ReactElement } from 'react';
import { AlertCircle, BookOpenCheck, ChevronRight, LogOut, MessageSquareText, Moon, Search, Sun } from 'lucide-react';
import {
  displayValue,
  feedbackComments,
  formatScore,
  getColumnLabel,
  getDetailItems,
  getSummaryLabel,
  humanizeLabel,
  isActivityColumn,
  isAverageColumn,
  isGradeColumn,
  normalized,
  scoreTone,
  scoreToneFromRatio,
  shouldShowMainColumn,
} from './gradeUtils';
import type { Column, GradeTable, SessionUser, StudentStatus } from './types';

export function LoginView({
  matricula,
  setMatricula,
  loading,
  error,
  theme,
  setTheme,
  onSubmit,
}: {
  matricula: string;
  setMatricula: (value: string) => void;
  loading: boolean;
  error: string;
  theme: 'light' | 'dark';
  setTheme: (theme: 'light' | 'dark') => void;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <main className="shell login-shell">
      <ThemeButton theme={theme} setTheme={setTheme} />
      <section className="login-panel">
        <div className="brand-mark">
          <BookOpenCheck size={34} strokeWidth={2.2} />
        </div>
        <h1>dbBack</h1>
        <p>Use sua matricula da UFAL para acessar suas notas e feedbacks das atividades.</p>
        <form onSubmit={onSubmit}>
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

export function Topbar({
  session,
  theme,
  setTheme,
  onLogout,
}: {
  session: SessionUser;
  theme: 'light' | 'dark';
  setTheme: (theme: 'light' | 'dark') => void;
  onLogout: () => void;
}) {
  return (
    <header className="topbar">
      <div>
        <span>{session.matricula}</span>
        <strong>{session.name || 'Aluno'}</strong>
      </div>
      <div className="topbar-actions">
        <button className="icon-button danger-button" type="button" onClick={onLogout} aria-label="Sair">
          <LogOut size={18} />
        </button>
        <ThemeButton theme={theme} setTheme={setTheme} compact />
      </div>
    </header>
  );
}

export function ExamSwitch({ exam, setExam }: { exam: 'ab1' | 'ab2'; setExam: (exam: 'ab1' | 'ab2') => void }) {
  return (
    <section className="exam-switch" aria-label="Selecionar avaliacao">
      <button className={exam === 'ab1' ? 'active' : ''} type="button" onClick={() => setExam('ab1')}>
        AB1
      </button>
      <button className={exam === 'ab2' ? 'active' : ''} type="button" onClick={() => setExam('ab2')}>
        AB2
      </button>
    </section>
  );
}

export function AB2Summary({ status }: { status: StudentStatus }) {
  return (
    <article className="grade-table summary">
      <header>
        <h2>Média AB2</h2>
      </header>
      <div className="summary-grid">
        <section className={`summary-score highlight ${status.ab2 < 5 ? 'score-danger' : status.ab2 < 7 ? 'score-warning' : 'score-success'}`}>
          <span>Média da AB</span>
          <strong>{formatScore(status.ab2)}</strong>
        </section>
      </div>
    </article>
  );
}

export function GradeCard({
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
      {table.columns.filter((column) => shouldShowMainColumn(column)).map((column) => (
        <div key={`${table.key}-${column.key}`}>
          <GradeRow column={column} expanded={activeKey === column.key} onToggle={() => onToggleDetail(table.key, column.key)} />
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

export function SummaryTable({ table, exam, status }: { table: GradeTable; exam: 'ab1' | 'ab2'; status: StudentStatus | null }) {
  const sortedColumns = [...table.columns.filter(shouldShowMainColumn)].sort((a, b) => {
    const labelA = normalized(a.label);
    const labelB = normalized(b.label);

    if (labelA.includes('prova') && !labelB.includes('prova')) return -1;
    if (!labelA.includes('prova') && labelB.includes('prova')) return 1;
    if (isAverageColumn(a) && !isAverageColumn(b)) return -1;
    if (!isAverageColumn(a) && isAverageColumn(b)) return 1;
    if (isActivityColumn(a) && !isActivityColumn(b)) return -1;
    if (!isActivityColumn(a) && isActivityColumn(b)) return 1;
    return labelA.localeCompare(labelB, 'pt-BR', { numeric: true });
  });

  return (
    <article className="grade-table summary">
      <header>
        <h2>{table.label}</h2>
      </header>
      <div className="summary-grid">
        {(() => {
          let insertedAvg = false;
          return sortedColumns.flatMap((column) => {
            const isAverage = isAverageColumn(column);
            const isAT4 = /\bat\.?\s*4\b/.test(normalized(column.label)) || normalized(column.label).includes('atividade 4');
            const parts: ReactElement[] = [
              <section className={`summary-score ${isAverage ? 'highlight' : ''} ${isAverage ? scoreTone(column) : ''}`} key={`${table.key}-${column.key}`}>
                <div className="summary-score-title">
                  <span>{getSummaryLabel(column)}</span>
                  {isAT4 && exam === 'ab2' ? <span className="at4-badge">AT.4</span> : null}
                </div>
                <strong>{displayValue(column)}</strong>
                {column.comment && (
                  <p>
                    <MessageSquareText size={15} />
                    {column.comment}
                  </p>
                )}
              </section>,
            ];

            if (!insertedAvg && normalized(column.label).includes('prova') && status) {
              const avgValue = exam === 'ab1' ? status.ab1 : exam === 'ab2' ? status.ab2 : null;
              if (avgValue !== null && avgValue !== undefined) {
                insertedAvg = true;
                parts.push(
                  <section className="summary-score highlight" key={`${table.key}-ab-average`}>
                    <span>{`Média ${exam.toUpperCase()}`}</span>
                    <strong>{formatScore(avgValue)}</strong>
                  </section>,
                );
              }
            }

            return parts;
          });
        })()}
      </div>
    </article>
  );
}

export function ThemeButton({
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

export function InlineError({ message }: { message: string }) {
  return (
    <div className="inline-error" role="alert">
      <AlertCircle size={18} />
      <span>{message}</span>
    </div>
  );
}

function GradeRow({ column, expanded, onToggle }: { column: Column; expanded: boolean; onToggle: () => void }) {
  const clickable = isGradeColumn(column);
  return (
    <section className={`grade-row ${expanded ? 'expanded' : ''} ${clickable ? 'clickable' : ''} ${scoreTone(column)}`}>
      <button type="button" className="grade-row-trigger" onClick={clickable ? onToggle : undefined} aria-expanded={expanded} disabled={!clickable}>
        <div>
          <span>{getColumnLabel(column)}</span>
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
  const parsedItems = getDetailItems(table, mainColumn);
  const isAt4Detail = /\bat\.?\s*4\b/.test(normalized(mainColumn.label)) || normalized(mainColumn.label).includes('atividade 4');
  const detailTitle = isAt4Detail ? `Detalhes ${humanizeLabel(mainColumn.label)}` : 'Composição';
  const mainComment = mainColumn.comment?.trim();

  return (
    <section className="detail-panel">
      <div className="detail-header">
        <div>
          <span>{detailTitle}</span>
          <strong>{mainColumn.label}</strong>
        </div>
        <div className="detail-score">
          {displayValue(mainColumn)}
          <small>nota total</small>
        </div>
      </div>
      <div className="detail-items">
        {parsedItems.map((item) => (
          <article className={`detail-item ${scoreToneFromRatio(item.ratio)}`} key={item.key}>
            <div className="detail-item-row">
              <div>
                <strong>{item.label}</strong>
              </div>
              {item.max ? <span className="badge">{item.obtained !== null ? `${formatScore(item.obtained)} / ${formatScore(item.max)}` : `Max ${formatScore(item.max)}`}</span> : null}
            </div>
            <div className="detail-progress-bar" aria-hidden="true">
              <div className="detail-progress-fill" style={{ width: `${item.ratio}%` }} />
            </div>
            {item.comment ? (
              <p className="detail-item-comment">
                <MessageSquareText size={14} />
                {item.comment}
              </p>
            ) : null}
          </article>
        ))}
      </div>
      {mainComment && (
        <div className="comment-bubble">
          <div className="comment-avatar">P</div>
          <div>
            <p>{mainComment}</p>
            <span>Comentário geral</span>
          </div>
        </div>
      )}
    </section>
  );
}
