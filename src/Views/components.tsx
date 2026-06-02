import { AlertCircle, BookOpenCheck, ChevronRight, LogOut, MessageSquareText, Moon, Search, Sun } from 'lucide-react';
import type { CSSProperties, FormEvent } from 'react';
import { useEffect, useRef } from 'react';
import { cardsFor, isMediaTable } from '../Models/gradeModel';
import { appVersion } from '../Models/version';
import type { GradeCard as GradeCardData, GradeDetail, GradeTable, SessionUser } from '../Models/types';

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
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
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
        <form onSubmit={onSubmit} autoComplete="on">
          <label htmlFor="matricula">Matricula UFAL</label>
          <div className="field">
            <Search size={18} />
            <input
              id="matricula"
              name="username"
              type="text"
              inputMode="numeric"
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              enterKeyHint="go"
              placeholder="Digite sua matricula"
              required
              value={matricula}
              onChange={(event) => setMatricula(event.target.value)}
            />
          </div>
          {error && <InlineError message={error} />}
          <button className="primary-button" type="submit" disabled={loading} aria-busy={loading}>
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
        {appVersion.stable && <span className="version-badge">{appVersion.label}</span>}
      </div>
      <div className="topbar-actions">
        <ThemeButton theme={theme} setTheme={setTheme} compact />
        <button className="icon-button danger-button" type="button" onClick={onLogout} aria-label="Sair">
          <LogOut size={18} />
        </button>
      </div>
    </header>
  );
}

export function ExamSwitch({
  exam,
  exams,
  labels,
  carousel,
  setExam,
}: {
  exam: string;
  exams: string[];
  labels: Record<string, string>;
  carousel?: boolean;
  setExam: (exam: string) => void;
}) {
  if (exams.length === 0) return null;
  const mode = exams.length === 1 ? 'single' : carousel || exams.length > 2 ? 'carousel' : 'pair';
  return (
    <section className="exam-switch" data-mode={mode} aria-label="Selecionar avaliacao">
      {exams.map((option) => (
        <button className={exam === option ? 'active' : ''} type="button" onClick={() => setExam(option)} aria-pressed={exam === option} key={option}>
          {labels[option] || option.toUpperCase()}
        </button>
      ))}
    </section>
  );
}

export function GradeCard({
  table,
  activeDetail,
  onToggleDetail,
  onPrefetch,
}: {
  table: GradeTable;
  activeDetail: { tableKey: string; cardKey: string } | null;
  onToggleDetail: (tableKey: string, cardKey: string) => void;
  onPrefetch?: () => void;
}) {
  const activeKey = activeDetail?.tableKey === table.key ? activeDetail.cardKey : null;
  const cards = cardsFor(table);
  const activeCard = cards.find((card) => card.key === activeKey);
  return (
    <article className={`grade-table ${table.kind} ${activeCard ? 'activity-open' : ''}`}>
      {activeCard ? (
        <OpenGradeCard table={table} cards={cards} activeCard={activeCard} onToggleDetail={onToggleDetail} onPrefetch={onPrefetch} />
      ) : (
        <>
          <GradeTableHeader table={table} />
          {cards.map((card) => (
            <GradeRow tableKey={table.key} card={card} expanded={false} onToggle={() => onToggleDetail(table.key, card.key)} onPrefetch={onPrefetch} key={`${table.key}-${card.key}`} />
          ))}
        </>
      )}
    </article>
  );
}

function OpenGradeCard({
  table,
  cards,
  activeCard,
  onToggleDetail,
  onPrefetch,
}: {
  table: GradeTable;
  cards: GradeCardData[];
  activeCard: GradeCardData;
  onToggleDetail: (tableKey: string, cardKey: string) => void;
  onPrefetch?: () => void;
}) {
  const inactiveCards = cards.filter((card) => card.key !== activeCard.key);
  return (
    <>
      <ActivityStickyBlock table={table} card={activeCard} onToggleDetail={onToggleDetail} onPrefetch={onPrefetch} />
      <GradeDetailPanel tableKey={table.key} card={activeCard} />
      {inactiveCards.map((card) => (
        <GradeRow tableKey={table.key} card={card} expanded={false} onToggle={() => onToggleDetail(table.key, card.key)} onPrefetch={onPrefetch} key={`${table.key}-${card.key}`} />
      ))}
    </>
  );
}

function ActivityStickyBlock({
  table,
  card,
  onToggleDetail,
  onPrefetch,
}: {
  table: GradeTable;
  card: GradeCardData;
  onToggleDetail: (tableKey: string, cardKey: string) => void;
  onPrefetch?: () => void;
}) {
  return (
    <div className="activity-sticky-block" data-detail-sticky>
      <GradeTableHeader table={table} />
      <GradeRow tableKey={table.key} card={card} expanded onToggle={() => onToggleDetail(table.key, card.key)} onPrefetch={onPrefetch} />
    </div>
  );
}

export function SummaryTable({ table }: { table: GradeTable }) {
  const cards = cardsFor(table);
  const firstTone = cards.find((card) => card.tone)?.tone || '';

  return (
    <article className={`grade-table summary ${isMediaTable(table) ? `final-average ${firstTone}` : ''}`}>
      <header>
        <h2>{table.label}</h2>
      </header>
      {cards.length > 0 && (
        <div className="summary-grid">
          {cards.map((card) => (
            <SummaryScoreCard card={card} fallbackLabel={table.label} key={`${table.key}-${card.key}`} />
          ))}
        </div>
      )}
    </article>
  );
}

function SummaryScoreCard({ card, fallbackLabel }: { card: GradeCardData; fallbackLabel: string }) {
  return (
    <section className={`summary-score ${summaryHighlight(card) ? 'highlight' : ''} ${card.tone || ''}`}>
      <div className="summary-score-title">
        <span>{card.label || fallbackLabel}</span>
      </div>
      <strong>{card.displayValue}</strong>
      {card.comment && (
        <p>
          <MessageSquareText size={15} />
          <span>
            {card.commentAuthor && <strong>{card.commentAuthor}</strong>}
            {card.comment}
          </span>
        </p>
      )}
    </section>
  );
}

function GradeTableHeader({ table }: { table: GradeTable }) {
  return (
    <header>
      <div>
        <h2>{table.label}</h2>
        {table.status && <span className="table-status">{table.status}</span>}
      </div>
    </header>
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

export function EmptyState({ exam }: { exam: string }) {
  return (
    <section className="empty-state" role="status">
      <BookOpenCheck size={24} />
      <div>
        <strong>{exam.toUpperCase()} ainda sem notas visíveis</strong>
        <p>Quando houver notas corrigidas para sua matrícula, elas aparecerão aqui.</p>
      </div>
    </section>
  );
}

function GradeRow({ tableKey, card, expanded, onToggle, onPrefetch }: { tableKey: string; card: GradeCardData; expanded: boolean; onToggle: () => void; onPrefetch?: () => void }) {
  const clickable = Boolean(card.details?.length);
  const panelId = detailPanelId(tableKey, card.key);
  return (
    <section className={`grade-row ${expanded ? 'expanded' : ''} ${clickable ? 'clickable' : ''} ${card.tone || ''}`}>
      <button
        type="button"
        className="grade-row-trigger"
        onClick={clickable ? onToggle : undefined}
        onMouseEnter={clickable ? onPrefetch : undefined}
        onFocus={clickable ? onPrefetch : undefined}
        aria-controls={clickable ? panelId : undefined}
        aria-expanded={clickable ? expanded : undefined}
        disabled={!clickable}
        aria-label={clickable ? `Abrir detalhes de ${card.label}` : `${card.label}: ${card.displayValue}`}
      >
        <div>
          <span>{card.label}</span>
          <strong>{card.displayValue}</strong>
        </div>
        {clickable && <ChevronRight size={18} className={expanded ? 'rotated' : ''} />}
      </button>
      {card.comment && (
        <p className="row-comment">
          <MessageSquareText size={15} />
          <span>
            {card.commentAuthor && <strong>{card.commentAuthor}</strong>}
            {card.comment}
          </span>
        </p>
      )}
    </section>
  );
}

function GradeDetailPanel({ tableKey, card }: { tableKey: string; card: GradeCardData }) {
  const details = card.details ?? [];
  const panelRef = useRef<HTMLElement>(null);

  useEffect(() => {
    if (details.length === 0) return;

    const frame = window.requestAnimationFrame(() => {
      const panel = panelRef.current;
      const detailHeading = panel?.querySelector<HTMLElement>('[data-detail-heading="true"]');
      if (!panel || !detailHeading) return;

      const prefersReducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
      detailHeading.scrollIntoView({ behavior: prefersReducedMotion ? 'auto' : 'smooth', block: 'start' });
    });

    return () => window.cancelAnimationFrame(frame);
  }, [tableKey, card.key, details.length]);

  return (
    <section className="detail-panel" id={detailPanelId(tableKey, card.key)} ref={panelRef}>
      <div className="detail-header">
        <div>
          <strong data-detail-heading="true">Critérios avaliados</strong>
        </div>
      </div>
      <DetailList details={details} />
      {card.comment && (
        <div className="comment-bubble">
          <div className="comment-avatar">P</div>
          <div>
            <p>{card.comment}</p>
            <span>{card.commentAuthor || 'Comentário geral'}</span>
          </div>
        </div>
      )}
    </section>
  );
}

function DetailList({ details }: { details: GradeDetail[] }) {
  return (
    <div className="detail-items">
      {details.map((item) => (
        <DetailItem item={item} key={item.key} />
      ))}
    </div>
  );
}

function DetailItem({ item }: { item: GradeDetail }) {
  return (
    <article className={`detail-item ${item.tone || ''}`}>
      <div className="detail-item-row">
        <div>
          <strong>{item.label}</strong>
        </div>
        <span className="badge">{item.displayScore}</span>
      </div>
      <ProgressBar value={item.ratio} />
      {item.comment ? (
        <p className="detail-item-comment">
          <MessageSquareText size={14} />
          <span>
            {item.commentAuthor && <strong>{item.commentAuthor}</strong>}
            {item.comment}
          </span>
        </p>
      ) : null}
    </article>
  );
}

function ProgressBar({ value }: { value: number }) {
  const progress = Math.min(Math.max(Number.isFinite(value) ? value : 0, 0), 100);
  return (
    <div className="progress-bar" aria-hidden="true" style={{ '--progress-value': `${progress}%` } as CSSProperties}>
      <div className="progress-fill" />
    </div>
  );
}

function summaryHighlight(card: GradeCardData) {
  const label = card.label.toLowerCase();
  return label.includes('total');
}

function detailPanelId(tableKey: string, cardKey: string) {
  return `details-${tableKey}-${cardKey}`;
}
