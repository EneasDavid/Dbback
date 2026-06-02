import { AlertCircle, BookOpenCheck, ChevronLeft, ChevronRight, LogOut, MessageSquareText, Moon, Search, Sun } from 'lucide-react';
import type { CSSProperties, FormEvent } from 'react';
import { useEffect, useRef, useState } from 'react';
import { cardsFor, isMediaTable } from '../Models/gradeModel';
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
  const switchRef = useRef<HTMLElement>(null);
  const optionRefs = useRef(new Map<string, HTMLButtonElement>());
  const [needsCarousel, setNeedsCarousel] = useState(false);
  const canCarousel = exams.length > 2 || Boolean(carousel);
  const mode = exams.length <= 1 ? 'single' : canCarousel && needsCarousel ? 'carousel' : exams.length === 2 ? 'pair' : 'fit';
  const activeIndex = Math.max(exams.indexOf(exam), 0);

  useEffect(() => {
    if (!canCarousel || exams.length <= 1) {
      setNeedsCarousel(false);
      return;
    }

    let frame = 0;
    const updateCarouselNeed = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(() => {
        const container = switchRef.current;
        const buttons = exams.map((option) => optionRefs.current.get(option));
        if (!container || buttons.some((button) => !button)) return;

        const styles = window.getComputedStyle(container);
        const padding = parseFloat(styles.paddingLeft) + parseFloat(styles.paddingRight);
        const availableWidth = container.clientWidth - padding;
        if (availableWidth <= 0) return;

        const gap = 6;
        const minimumOptionWidth = 124;
        const totalOptionWidth = buttons.reduce((total, button) => total + Math.max(button?.scrollWidth ?? 0, minimumOptionWidth), 0);
        setNeedsCarousel(totalOptionWidth + gap * (exams.length - 1) > availableWidth + 1);
      });
    };

    updateCarouselNeed();
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(updateCarouselNeed);
    if (observer && switchRef.current) {
      observer.observe(switchRef.current);
    }
    window.addEventListener('resize', updateCarouselNeed);

    return () => {
      window.cancelAnimationFrame(frame);
      observer?.disconnect();
      window.removeEventListener('resize', updateCarouselNeed);
    };
  }, [canCarousel, exams, labels]);

  useEffect(() => {
    if (mode !== 'carousel') return;
    const activeOption = optionRefs.current.get(exam);
    if (!activeOption) return;

    const prefersReducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
    activeOption.scrollIntoView({ behavior: prefersReducedMotion ? 'auto' : 'smooth', block: 'nearest', inline: 'center' });
  }, [exam, mode]);

  const chooseNearbyExam = (direction: -1 | 1) => {
    const nextIndex = (activeIndex + direction + exams.length) % exams.length;
    setExam(exams[nextIndex]);
  };

  if (exams.length === 0) return null;

  const optionButtons = exams.map((option) => (
    <button
      className={`exam-option ${exam === option ? 'active' : ''}`}
      type="button"
      onClick={() => setExam(option)}
      aria-pressed={exam === option}
      key={option}
      ref={(node) => {
        if (node) {
          optionRefs.current.set(option, node);
          return;
        }
        optionRefs.current.delete(option);
      }}
    >
      {labels[option] || option.toUpperCase()}
    </button>
  ));

  return (
    <section className="exam-switch" data-mode={mode} aria-label="Selecionar avaliacao" ref={switchRef}>
      {mode === 'carousel' ? (
        <>
          <button className="exam-nav" type="button" onClick={() => chooseNearbyExam(-1)} aria-label="Avaliação anterior" title="Anterior">
            <ChevronLeft size={18} />
          </button>
          <div className="exam-track">{optionButtons}</div>
          <button className="exam-nav" type="button" onClick={() => chooseNearbyExam(1)} aria-label="Próxima avaliação" title="Próxima">
            <ChevronRight size={18} />
          </button>
        </>
      ) : optionButtons}
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
  const title = isMediaTable(table) ? 'Média' : table.label;

  return (
    <article className={`grade-table summary ${isMediaTable(table) ? `final-average ${firstTone}` : ''}`}>
      <header>
        <h2>{title}</h2>
      </header>
      {cards.length > 0 && (
        <div className="summary-grid">
          {cards.map((card) => (
            <SummaryScoreCard card={card} fallbackLabel={title} key={`${table.key}-${card.key}`} />
          ))}
        </div>
      )}
    </article>
  );
}

export function ReaderGradeDocument({
  session,
  examLabel,
  tables,
}: {
  session: SessionUser;
  examLabel: string;
  tables: GradeTable[];
}) {
  const readableTables = tables
    .map((table) => ({ table, cards: cardsFor(table) }))
    .filter(({ cards }) => cards.length > 0);

  if (readableTables.length === 0) return null;

  return (
    <article className="reader-document" aria-labelledby="reader-document-title" itemScope itemType="https://schema.org/Article">
      <header>
        <p>Resumo em tópicos</p>
        <h1 id="reader-document-title">Notas e feedbacks - {examLabel}</h1>
        <dl>
          <div>
            <dt>Aluno</dt>
            <dd>{session.name || 'Aluno'}</dd>
          </div>
          <div>
            <dt>Matrícula</dt>
            <dd>{session.matricula}</dd>
          </div>
          <div>
            <dt>Avaliação</dt>
            <dd>{examLabel}</dd>
          </div>
        </dl>
      </header>

      {readableTables.map(({ table, cards }) => (
        <section className="reader-topic" key={`reader-${table.key}`}>
          <h2>{table.label}</h2>
          {table.status && <p>Status: {table.status}</p>}
          <ul>
            {cards.map((card) => (
              <li key={`reader-${table.key}-${card.key}`}>
                <h3>{card.label}: {card.displayValue}</h3>
                {card.comment && (
                  <p>
                    Feedback{card.commentAuthor ? ` de ${card.commentAuthor}` : ''}: {card.comment}
                  </p>
                )}
                {card.details && card.details.length > 0 && (
                  <ul>
                    {card.details.map((detail) => (
                      <li key={`reader-${table.key}-${card.key}-${detail.key}`}>
                        <p>
                          <strong>{detail.label}</strong>: {detail.displayScore}
                        </p>
                        <p>Progresso: {formatReaderPercent(detail.ratio)}</p>
                        {detail.comment && (
                          <p>
                            Feedback{detail.commentAuthor ? ` de ${detail.commentAuthor}` : ''}: {detail.comment}
                          </p>
                        )}
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            ))}
          </ul>
        </section>
      ))}
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
      const activeActivity = panel?.previousElementSibling instanceof HTMLElement ? panel.previousElementSibling : null;
      if (!panel || !activeActivity) return;

      const prefersReducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
      activeActivity.scrollIntoView({ behavior: prefersReducedMotion ? 'auto' : 'smooth', block: 'start' });
    });

    return () => window.cancelAnimationFrame(frame);
  }, [tableKey, card.key, details.length]);

  return (
    <section className="detail-panel" id={detailPanelId(tableKey, card.key)} ref={panelRef}>
      <div className="detail-header">
        <div>
          <strong>Critérios avaliados</strong>
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

function formatReaderPercent(value: number) {
  const percent = Math.min(Math.max(Number.isFinite(value) ? value : 0, 0), 100);
  return `${percent.toLocaleString('pt-BR', { maximumFractionDigits: 1 })}%`;
}

function detailPanelId(tableKey: string, cardKey: string) {
  return `details-${tableKey}-${cardKey}`;
}
