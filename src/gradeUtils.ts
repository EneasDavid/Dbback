import type { Column, GradeResult, GradeTable, StudentStatus } from './types';

export type DetailItem = {
  key: string;
  label: string;
  value: string;
  comment?: string;
  obtained: number | null;
  max: number;
  ratio: number;
};

export function computeStudentStatus(ab1: GradeResult, ab2: GradeResult): StudentStatus | null {
  const ab1Total = findScore(
    ab1,
    (table, column) => table.complete && table.kind === 'summary' && normalized(column.label).includes('nota') && normalized(column.label).includes('ab'),
  );
  const ab2Total = computeAB2Score(ab2);
  if (ab1Total === null || ab2Total === null) return null;
  const average = (ab1Total + ab2Total) / 2;
  return { ab1: ab1Total, ab2: ab2Total, average, approved: average >= 7 };
}

export function computeAB2Score(grade: GradeResult) {
  const at4 = findScore(grade, (table, column) => table.key === 'at4' && normalized(column.label) === 'nota');
  const project = findScore(grade, (table, column) => table.key === 'projeto' && normalized(column.label) === 'total');
  if (at4 === null || project === null) return null;
  return at4 + project;
}

export function getDetailItems(table: GradeTable, mainColumn: Column): DetailItem[] {
  if (table.items?.length) {
    return table.items
      .filter((item) => normalized(item.subtopic) !== 'total')
      .map((item) => {
        const obtained = parseScore(item.notaAlcancada);
        const max = parseScore(item.notaMaxima) ?? 1;
        const ratio = obtained !== null && max > 0 ? Math.min((obtained / max) * 100, 100) : 0;

        return {
          key: item.key,
          label: humanizeLabel(item.subtopic),
          value: item.notaAlcancada,
          comment: item.comentario,
          obtained,
          max,
          ratio,
        };
      });
  }

  const seen = new Set<string>();
  return table.columns
    .filter((item) => shouldShowColumn(item) && !shouldShowMainColumn(item) && item.key !== mainColumn.key && !isAverageColumn(item))
    .sort((a, b) => compareDetailItemOrder(a, b))
    .filter((item) => {
      const normalizedLabel = normalized(item.label);
      if (seen.has(normalizedLabel)) return false;
      seen.add(normalizedLabel);
      return true;
    })
    .map((item) => {
      const obtained = parseScore(item.value);
      const max = inferMaxForLabel(item.label) ?? 1;
      const ratio = obtained !== null && max > 0 ? Math.min((obtained / max) * 100, 100) : 0;

      return {
        key: item.key,
        label: getColumnLabel(item),
        value: item.value,
        comment: item.comment,
        obtained,
        max,
        ratio,
      };
    });
}

export function shouldShowColumn(column: Column) {
  const label = normalized(column.label);
  return label !== '' && label !== 'grupo' && label !== 'equipe' && !label.includes('matricula') && !label.includes('nome do aluno') && label !== 'nome' && label !== 'aluno';
}

export function shouldShowMainColumn(column: Column) {
  if (!shouldShowColumn(column) || isDetailOnlyColumn(column)) return false;
  const label = normalized(column.label);
  if (/\bat\.?\s*4\b/.test(label) || label.includes('atividade 4')) return true;
  return label === 'nota' || label.includes('prova') || label === 'total' || label.includes('média') || isActivityColumn(column) || label.includes('projeto') || label === 'ab1' || label === 'ab2';
}

export function isFinalAverageColumn(column: Column) {
  const label = normalized(column.label);
  return label.includes('nota') && label.includes('ab') && !label.includes('prova');
}

export function shouldShowTable(table: GradeTable) {
  return table.kind === 'summary' || table.columns.some(shouldShowMainColumn) || Boolean(table.items?.length) || feedbackComments(table).length > 0;
}

export function feedbackComments(table: GradeTable) {
  const seen = new Set<string>();
  const comments: string[] = [];
  for (const column of table.columns) {
    const comment = column.comment?.trim();
    if (comment && !seen.has(comment)) {
      seen.add(comment);
      comments.push(comment);
    }
  }
  for (const item of table.items ?? []) {
    const comment = item.comentario?.trim();
    if (comment && !seen.has(comment)) {
      seen.add(comment);
      comments.push(comment);
    }
  }
  return comments;
}

export function findScore(grade: GradeResult, predicate: (table: GradeTable, column: Column) => boolean) {
  for (const table of grade.tables ?? []) {
    for (const column of table.columns ?? []) {
      if (predicate(table, column)) {
        const score = parseScore(column.value);
        return score !== null ? score : null;
      }
    }
  }
  return null;
}

export function normalizeGrade(grade: GradeResult, fallbackExam: string): GradeResult {
  return {
    ...grade,
    exam: grade.exam || fallbackExam,
    tables: Array.isArray(grade.tables)
      ? grade.tables.map((table) => ({
          ...table,
          columns: Array.isArray(table.columns) ? table.columns : [],
          items: Array.isArray(table.items) ? table.items : [],
        }))
      : [],
  };
}

export function displayValue(column: Column) {
  if (isGradeColumn(column) && parseScore(column.value) === null) return 'Não corrigida ainda';
  return column.value || 'Não corrigida ainda';
}

export function scoreToneFromRatio(ratio: number) {
  if (ratio < 50) return 'score-danger';
  if (ratio < 70) return 'score-warning';
  return 'score-success';
}

export function scoreTone(column: Column) {
  const label = normalized(column.label);
  if (!label.includes('nota ab')) return '';
  const value = parseScore(column.value);
  if (value === null) return '';
  if (value < 5) return 'score-danger';
  if (value < 7) return 'score-warning';
  return 'score-success';
}

export function getSummaryLabel(column: Column) {
  const label = normalized(column.label);
  if (label.includes('prova')) return 'Nota da prova';
  if (isAverageColumn(column)) return 'Média da AB';
  if (isActivityColumn(column)) return label.toUpperCase().replace(/at\.?\s*/, 'AT. ');
  if (label === 'total') return 'Total';
  if (label.includes('projeto')) return 'Projeto';
  return humanizeLabel(column.label);
}

export function getColumnLabel(column: Column) {
  const label = normalized(column.label);
  if (label.startsWith('semana')) return humanizeLabel(column.label);
  if (label.startsWith('q.') || label.startsWith('q')) return label.toUpperCase().replace('Q.', 'Q.').replace('Q', 'Q.');
  if (label === 'sgbd') return 'SGBD';
  if (label === 'dataset') return 'Dataset';
  if (label === 'crud') return 'CRUD';
  if (label.includes('apresentacao')) return 'Apresentação';
  if (label.includes('organizacao') || label.includes('organização')) return 'Organização';
  if (label.includes('referencias')) return 'Referências';
  if (label.includes('discussao')) return 'Discussão em aula';
  return humanizeLabel(column.label);
}

export function formatScore(value: number) {
  return value.toLocaleString('pt-BR', { maximumFractionDigits: 2, minimumFractionDigits: value % 1 === 0 ? 0 : 1 });
}

export function parseScore(value: string) {
  const parsed = Number(value.replace(',', '.').replace(/[^\d.-]/g, ''));
  return Number.isFinite(parsed) ? parsed : null;
}

export function normalized(value: string) {
  return value
    .toLowerCase()
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .trim();
}

function isDetailOnlyColumn(column: Column) {
  const label = normalized(column.label);
  return label.startsWith('semana') || label === 'sgbd' || label === 'dataset' || label === 'crud' || label.includes('apresentacao') || label.includes('organizacao') || label.includes('organização') || label.includes('q.') || label.startsWith('q');
}

export function isActivityColumn(column: Column) {
  const label = normalized(column.label);
  return /^at\.?\s*\d+/i.test(label) || /atividade/.test(label) || /at\.?\s*4/.test(label);
}

export function isAverageColumn(column: Column) {
  const label = normalized(column.label);
  return label.includes('média') || label.includes('media') || label.includes('média da ab') || label.includes('media da ab');
}

export function isGradeColumn(column: Column) {
  const label = normalized(column.label);
  return label === 'nota' || label.includes('prova') || label.includes('nota ab') || label === 'total' || label.startsWith('semana') || ['sgbd', 'dataset', 'crud', 'apresentacao', 'projeto'].includes(label) || isActivityColumn(column);
}

export function humanizeLabel(label: string) {
  return label
    .replace(/\b(at\.?\s*\d+)\b/i, (match) => match.toUpperCase().replace('AT', 'AT.'))
    .replace(/\bq\.?\s*(\d+)\b/i, (match) => match.toUpperCase().replace('Q', 'Q.'))
    .replace(/\bsgbd\b/i, 'SGBD')
    .replace(/\bdataset\b/i, 'Dataset')
    .replace(/\bcrud\b/i, 'CRUD')
    .replace(/\bapresentacao\b/i, 'Apresentação')
    .replace(/\borganizacao\b/i, 'Organização')
    .replace(/\breferencias\b/i, 'Referências')
    .replace(/\bdiscussao\b/i, 'Discussão')
    .replace(/\bnota\b/i, 'Nota')
    .replace(/\bm[ée]dia\b/i, 'Média')
    .replace(/\btotal\b/i, 'Total')
    .replace(/\bsemana\b/i, 'Semana')
    .replace(/\s+/g, ' ')
    .trim();
}

function inferMaxForLabel(label: string) {
  const l = normalized(label);
  const map: Record<string, number> = {
    organizacao: 0.5,
    organização: 0.5,
    'q.1': 1.5,
    'q.2': 1,
    'q.3': 1.5,
    'q.4': 2,
    'q.5': 1.5,
    'q.6': 2,
    'semana 1': 0.25,
    'semana 2': 0.25,
    'semana 3': 0.25,
    'semana 4': 0.25,
    sgbd: 1,
    dataset: 1,
    crud: 1,
    apresentacao: 2,
    apresentação: 2,
  };
  for (const key of Object.keys(map)) {
    if (l.includes(key)) return map[key];
  }
  return null;
}

function compareDetailItemOrder(a: Column, b: Column) {
  const order = ['organizacao', 'organização', 'q.1', 'q.2', 'q.3', 'q.4', 'q.5', 'q.6', 'semana 1', 'semana 2', 'semana 3', 'semana 4', 'sgbd', 'dataset', 'crud', 'apresentacao', 'referencias', 'discussao'];
  const labelA = normalized(a.label);
  const labelB = normalized(b.label);
  const indexA = order.findIndex((item) => labelA.includes(item));
  const indexB = order.findIndex((item) => labelB.includes(item));
  if (indexA !== indexB) {
    if (indexA === -1) return 1;
    if (indexB === -1) return -1;
    return indexA - indexB;
  }
  return labelA.localeCompare(labelB, 'pt-BR', { numeric: true });
}
